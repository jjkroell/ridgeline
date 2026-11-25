package store

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

// SOSChannel is the mesh's emergency hashtag channel, without its '#'. It is
// always decrypted (registered at startup whether or not discovery has
// confirmed it) and every browser carries it as a permanent channel.
//
// Who monitors it is what people say: a signed-in account switches "I'm
// monitoring #sos" on and gives the name of its companion radio, which must be
// one the mesh has heard (see SetSOSMonitor, FindCompanion). Until
// v0.24.78 the list was learned from "join" and "leave" messages sent on the
// channel; those are now ordinary messages (the old channel_monitors rows stay
// in databases that have them, unread). The web UI still hides join, leave and
// test messages from the #sos chat after five minutes (other messages after 3
// hours, calls for help after 12; sos.svelte.ts).
const SOSChannel = "sos"

// RegisterSOSChannel adds #sos to the decoder's channel set. Idempotent;
// called at startup before ingest so live #sos messages decrypt from the first
// packet, on a database where discovery has never confirmed the name too.
func RegisterSOSChannel() {
	meshcore.RegisterChannel(SOSChannel, meshcore.DeriveHashtagKey(SOSChannel))
}

// SOSMonitor is one account listed as monitoring #sos, under the radio name it gave.
type SOSMonitor struct {
	UserID int64  `json:"-"`
	Name   string `json:"name"`
	Since  string `json:"since"` // RFC3339, when it was switched on
}

// sosPinnedMonitor leads the list whenever it is on it: the bot that answers
// calls on #sos.
const sosPinnedMonitor = "KOD-BOT"

// ErrSOSNameTaken: another account is already listed under that name.
var ErrSOSNameTaken = errors.New("store: #sos monitor name taken")

// sosAlertRe matches an emergency word: sos, emergency, mayday or 911 as a
// whole word ("#sos", the channel's own tag, does not count), or "need help" /
// "help me" (a bare "help" — "thanks for the help" — does not). Mirrored by
// ALERT_RE in web/src/lib/sos.svelte.ts; keep the two in step. Go's RE2 has no
// lookbehind, so the '#' and word-boundary tests on the left are a group.
var sosAlertRe = regexp.MustCompile(`(?i)(?:^|[^#\w])(?:sos|emergency|mayday|911|threat|life)(?:\W|$)|\bneeds? help\b|\bhelp me\b`)

// sosReplyRe matches a message that opens with an @-mention ("@[Dave] on my
// way"): a reply in a conversation, not a new call for help, so it raises no
// alert even when it uses an emergency word. Mirrored by REPLY_RE in
// sos.svelte.ts (and the KOD-BOT SosAck plugin).
var sosReplyRe = regexp.MustCompile(`^\s*@\[`)

// sosAlertText reports whether a #sos message body is a call for help: an
// emergency word, and not a reply to someone.
func sosAlertText(msg string) bool {
	return sosAlertRe.MatchString(msg) && !sosReplyRe.MatchString(msg)
}

// SOSAlertWindow is how far back an alert keeps the #sos channel flagged in
// the UI (the channel row and the Channels nav item pulse), unless the viewer
// opens #sos first. The client applies it; the server reports the newest alert.
const SOSAlertWindow = 12 * time.Hour

// isSOSAlert reports whether a decrypted #sos message carries an emergency word.
func isSOSAlert(p *meshcore.Packet) bool {
	return p != nil && p.GroupText != nil && p.GroupText.Decrypted &&
		p.GroupText.Channel == SOSChannel && sosAlertText(p.GroupText.Message)
}

// SOSAlertMsg is a newly heard #sos message carrying an emergency word, handed
// to the alert hook (the email notifier) once per transmission.
type SOSAlertMsg struct {
	Hash       string
	Sender     string
	Message    string
	ReceivedAt time.Time
}

// SetSOSAlertHook registers fn to be called, after the write commits, the
// first time a #sos alert transmission is stored — never for another
// observer's copy or a late copy (the message hash is already stored), nor for
// the startup scan. fn must not block: Record holds the write lock.
func (s *Store) SetSOSAlertHook(fn func(SOSAlertMsg)) {
	s.sosMu.Lock()
	s.sosAlertHook = fn
	s.sosMu.Unlock()
}

// SOSAlertSender is who sent a #sos alert and when (RFC3339), newest per
// sender. The maps put a pulsing ring on that sender's node, if it has a
// position; the name is all a channel message carries, so the client matches it.
type SOSAlertSender struct {
	Sender string `json:"sender"`
	At     string `json:"at"`
}

// noteSOSAlert remembers the newest #sos alert time (RFC3339) and who sent it.
func (s *Store) noteSOSAlert(at, sender string) {
	s.sosMu.Lock()
	if at > s.sosLastAlert {
		s.sosLastAlert = at
	}
	if sender = strings.TrimSpace(sender); sender != "" {
		found := false
		for i := range s.sosAlerts {
			if s.sosAlerts[i].Sender == sender {
				found = true
				if at > s.sosAlerts[i].At {
					s.sosAlerts[i].At = at
				}
			}
		}
		if !found {
			s.sosAlerts = append(s.sosAlerts, SOSAlertSender{Sender: sender, At: at})
		}
	}
	s.sosMu.Unlock()
}

// SOSAlertSenders returns who sent #sos alerts within SOSAlertWindow of now,
// newest first, leaving out alerts an admin has since resolved. Served from
// memory; older entries are dropped as it goes.
func (s *Store) SOSAlertSenders(now time.Time) []SOSAlertSender {
	cutoff := now.UTC().Add(-SOSAlertWindow).Format(time.RFC3339Nano)
	s.sosMu.Lock()
	defer s.sosMu.Unlock()
	kept := s.sosAlerts[:0]
	for _, a := range s.sosAlerts {
		if a.At >= cutoff {
			kept = append(kept, a)
		}
	}
	s.sosAlerts = kept
	var out []SOSAlertSender
	for _, a := range kept {
		if a.At > s.sosResolved[a.Sender] {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At > out[j].At })
	return out
}

// ResolveSOSAlert clears the map ring for sender's alerts for everyone (an
// admin's "resolved"). A later alert from them rings again.
func (s *Store) ResolveSOSAlert(sender string, userID int64, now time.Time) error {
	sender = strings.TrimSpace(sender)
	at := now.UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(`INSERT INTO sos_resolved (sender, resolved_at, user_id) VALUES (?, ?, ?)
		ON CONFLICT(sender) DO UPDATE SET resolved_at = excluded.resolved_at, user_id = excluded.user_id`,
		sender, at, userID); err != nil {
		return err
	}
	s.sosMu.Lock()
	s.sosResolved[sender] = at
	s.sosMu.Unlock()
	return nil
}

// loadSOSResolved fills the resolved map at open (a handful of rows).
func (s *Store) loadSOSResolved() error {
	rows, err := s.db.Query(`SELECT sender, resolved_at FROM sos_resolved`)
	if err != nil {
		return err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var sender, at string
		if err := rows.Scan(&sender, &at); err != nil {
			return err
		}
		m[sender] = at
	}
	s.sosMu.Lock()
	s.sosResolved = m
	s.sosMu.Unlock()
	return rows.Err()
}

// SOSLastAlert returns when the newest #sos alert still in need of attention
// was heard ("" if none since startup's scan): an alert an admin has resolved
// no longer counts, so resolving everyone's alerts stops the channel pulse for
// everyone. Served from memory.
func (s *Store) SOSLastAlert() string {
	s.sosMu.RLock()
	defer s.sosMu.RUnlock()
	newest, open := "", ""
	for _, a := range s.sosAlerts {
		newest = max(newest, a.At)
		if a.At > s.sosResolved[a.Sender] {
			open = max(open, a.At)
		}
	}
	// sosAlerts holds the newest alert per named sender, so it covers
	// sosLastAlert unless that alert had no sender name — which can't be
	// resolved, so it keeps counting.
	if newest != s.sosLastAlert {
		return s.sosLastAlert
	}
	return open
}

// SOSAlertsClearedAt returns the newest alert time an admin has resolved (""
// if none). The client's own instant check of the #sos messages it holds
// stops there, so a resolved alert doesn't pulse again from the page's copy.
func (s *Store) SOSAlertsClearedAt() string {
	s.sosMu.RLock()
	defer s.sosMu.RUnlock()
	cleared := ""
	for _, a := range s.sosAlerts {
		if a.At <= s.sosResolved[a.Sender] {
			cleared = max(cleared, a.At)
		}
	}
	return cleared
}

// LoadSOSAlert finds the #sos alerts in the last SOSAlertWindow — the newest
// time and who sent them — so the flag and the map rings survive a restart.
// Runs every boot: twelve hourly batches over idx_obs_received (~a few
// thousand GroupText rows each), each its own short query so ingest isn't held
// up. Live ingest may note a newer alert meanwhile; noteSOSAlert keeps the max.
func (s *Store) LoadSOSAlert(ctx context.Context, now time.Time) error {
	end := now.UTC()
	for to := end; to.After(end.Add(-SOSAlertWindow)); to = to.Add(-time.Hour) {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := s.db.Query(`
			SELECT raw_hex, received_at FROM observations
			WHERE received_at >= ? AND received_at < ? AND payload_type = 'GroupText'`,
			to.Add(-time.Hour).Format(time.RFC3339Nano), to.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		for rows.Next() {
			var raw, at string
			if err := rows.Scan(&raw, &at); err != nil {
				rows.Close()
				return err
			}
			if p, err := meshcore.DecodeHex(raw); err == nil && isSOSAlert(p) {
				s.noteSOSAlert(at, p.GroupText.Sender)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}
	return nil
}

// SetSOSMonitor lists an account as monitoring #sos under name, or takes it
// off. Off keeps the saved name for next time. A name another listed account
// uses, in any case, is refused, so nobody can be listed as someone else who is
// already there.
func (s *Store) SetSOSMonitor(id int64, on bool, name string) error {
	s.sosRosterMu.Lock()
	defer s.sosRosterMu.Unlock()
	if !on {
		if _, err := s.db.Exec(`UPDATE users SET sos_monitor = 0 WHERE id = ?`, id); err != nil {
			return err
		}
		return s.readSOSMonitors()
	}
	for _, m := range s.SOSMonitors() {
		if m.UserID != id && strings.EqualFold(m.Name, name) {
			return ErrSOSNameTaken
		}
	}
	// "Since" is when it was switched on; a rename while on keeps it.
	if _, err := s.db.Exec(`UPDATE users SET
			sos_monitor_at = CASE WHEN sos_monitor = 1 THEN sos_monitor_at ELSE ? END,
			sos_monitor = 1, sos_monitor_name = ?
		WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), name, id); err != nil {
		return err
	}
	return s.readSOSMonitors()
}

// FindCompanion returns the advertised name of the heard companion radio called
// name, ok=false if there is none. Case is ignored; so are spaces, punctuation
// and emoji when that leaves exactly one companion name (they are hard to
// type). Companions not heard within retention, retired or blocked don't count.
func (s *Store) FindCompanion(name string) (string, bool, error) {
	name = strings.TrimSpace(name)
	rows, err := s.db.Query(`SELECT pubkey, name FROM nodes
		WHERE role = 'ChatNode' AND COALESCE(name,'') <> '' AND COALESCE(retired_at,'') = ''`)
	if err != nil {
		return "", false, err
	}
	defer rows.Close()
	plain, near := plainName(name), map[string]bool{}
	exact := ""
	for rows.Next() {
		var pk, n string
		if err := rows.Scan(&pk, &n); err != nil {
			return "", false, err
		}
		if s.IsNodeBlocked(pk) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(n), name) {
			exact = n
		}
		if plain != "" && plainName(n) == plain {
			near[n] = true
		}
	}
	if err := rows.Err(); err != nil {
		return "", false, err
	}
	if exact != "" {
		return strings.TrimSpace(exact), true, nil
	}
	if len(near) == 1 {
		for n := range near {
			return strings.TrimSpace(n), true, nil
		}
	}
	return "", false, nil
}

// plainName is a name's letters and digits, lowercased.
func plainName(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, name)
}

// loadSOSMonitors fills the in-memory roster at open, and again when an
// account that may be on it is blocked, unblocked or removed.
func (s *Store) loadSOSMonitors() error {
	s.sosRosterMu.Lock()
	defer s.sosRosterMu.Unlock()
	return s.readSOSMonitors()
}

// readSOSMonitors re-reads the roster; the caller holds sosRosterMu, so an
// older read can't replace a newer one.
func (s *Store) readSOSMonitors() error {
	rows, err := s.db.Query(`SELECT id, sos_monitor_name, sos_monitor_at FROM users
		WHERE sos_monitor = 1 AND blocked = 0 AND sos_monitor_name <> ''`)
	if err != nil {
		return err
	}
	defer rows.Close()
	mons := []SOSMonitor{}
	for rows.Next() {
		var m SOSMonitor
		if err := rows.Scan(&m.UserID, &m.Name, &m.Since); err != nil {
			return err
		}
		mons = append(mons, m)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	sort.Slice(mons, func(i, j int) bool {
		if a, b := mons[i].Name == sosPinnedMonitor, mons[j].Name == sosPinnedMonitor; a != b {
			return a
		}
		if a, b := strings.ToLower(mons[i].Name), strings.ToLower(mons[j].Name); a != b {
			return a < b
		}
		return mons[i].UserID < mons[j].UserID
	})
	s.sosMu.Lock()
	s.sosMonitors = mons
	s.sosMu.Unlock()
	return nil
}

// SOSMonitors returns who monitors #sos, by name (sosPinnedMonitor first). Served from memory — the
// endpoint never queues on the single DB connection. Never nil.
func (s *Store) SOSMonitors() []SOSMonitor {
	s.sosMu.RLock()
	defer s.sosMu.RUnlock()
	return append([]SOSMonitor{}, s.sosMonitors...)
}
