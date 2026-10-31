package store

import (
	"context"
	"database/sql"
	"fmt"
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
// Who monitors it is learned from the channel itself: a sender who posts a
// message starting with the word "test" is listed as a monitor, so the list
// says who has shown they are listening rather than who signed up somewhere;
// one starting with "unsub" (or "unsubscribe") takes them off it again. The
// web UI hides both kinds of message from the #sos chat after five minutes.
// The sender is the name the message carries — MeshCore group text is not
// signed per sender, so the list is a courtesy roster, not an identity proof.
const SOSChannel = "sos"

// sosBackfillMarker records that history was scanned once for monitors, in
// channel_monitor_backfills, so the 45-day scan runs on the first boot only.
const sosBackfillMarker = SOSChannel

// RegisterSOSChannel adds #sos to the decoder's channel set. Idempotent;
// called at startup before ingest so live #sos messages decrypt from the first
// packet, on a database where discovery has never confirmed the name too.
func RegisterSOSChannel() {
	meshcore.RegisterChannel(SOSChannel, meshcore.DeriveHashtagKey(SOSChannel))
}

// ChannelMonitor is one sender who has sent a test message to a channel.
type ChannelMonitor struct {
	Name      string `json:"name"`
	FirstTest string `json:"firstTest"` // RFC3339, server receive time
	LastTest  string `json:"lastTest"`
	Tests     int    `json:"tests"` // distinct test messages (not observations)
	// UnsubAt is the newest unsub; the sender is listed only while LastTest is
	// newer. Times rather than arrival order decide it, so a late copy of an
	// old test (the holding pen stores late) cannot re-list someone.
	UnsubAt string `json:"-"`
}

// active reports whether the sender is currently listed.
func (m *ChannelMonitor) active() bool { return m.LastTest != "" && m.LastTest > m.UnsubAt }

type monitorKind int

const (
	notMonitor monitorKind = iota
	monitorTest
	monitorUnsub
)

// monitorWord classifies a channel message by its first word, in any case,
// with trailing punctuation allowed: "test" ("Test!", "test from Nanaimo",
// "TEST 1") checks in; "unsub" or "unsubscribe" checks out. "testing" and
// "contest" are neither. Mirrored in web/src/lib/sos.svelte.ts.
func monitorWord(msg string) monitorKind {
	f := strings.Fields(msg)
	if len(f) == 0 {
		return notMonitor
	}
	w := strings.ToLower(strings.TrimRightFunc(f[0], func(r rune) bool { return unicode.IsPunct(r) }))
	switch w {
	case "test":
		return monitorTest
	case "unsub", "unsubscribe":
		return monitorUnsub
	}
	return notMonitor
}

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

// sosMonitorMsg returns the sender and kind of a decrypted #sos test or unsub.
func sosMonitorMsg(p *meshcore.Packet) (string, monitorKind) {
	if p == nil || p.GroupText == nil || !p.GroupText.Decrypted || p.GroupText.Channel != SOSChannel {
		return "", notMonitor
	}
	k := monitorWord(p.GroupText.Message)
	sender := strings.TrimSpace(p.GroupText.Sender)
	if k == notMonitor || sender == "" {
		return "", notMonitor
	}
	return sender, k
}

type execer interface {
	Exec(string, ...any) (sql.Result, error)
}

// writeMonitor records one test or unsub. For a test, lastHash dedupes the
// copies of a single transmission (re-floods and other observers arrive within
// seconds), so `tests` counts messages; every time takes a min/max so the
// backfill and live ingest can write in either order. An unsub only moves
// unsub_at forward; it keeps the row so its time can outrank late copies.
func writeMonitor(ex execer, kind monitorKind, channel, sender, hash, at string) error {
	if kind == monitorUnsub {
		_, err := ex.Exec(`
			INSERT INTO channel_monitors (channel, sender, first_test, last_test, tests, last_hash, unsub_at)
			VALUES (?,?,'','',0,'',?)
			ON CONFLICT(channel, sender) DO UPDATE SET
				unsub_at = MAX(channel_monitors.unsub_at, excluded.unsub_at)`,
			channel, sender, at)
		return err
	}
	_, err := ex.Exec(`
		INSERT INTO channel_monitors (channel, sender, first_test, last_test, tests, last_hash)
		VALUES (?,?,?,?,1,?)
		ON CONFLICT(channel, sender) DO UPDATE SET
			first_test = CASE WHEN channel_monitors.first_test = '' THEN excluded.first_test
			                  ELSE MIN(channel_monitors.first_test, excluded.first_test) END,
			last_test  = MAX(channel_monitors.last_test, excluded.last_test),
			tests      = channel_monitors.tests + CASE WHEN channel_monitors.last_hash = excluded.last_hash THEN 0 ELSE 1 END,
			last_hash  = excluded.last_hash`,
		channel, sender, at, at, hash)
	return err
}

// noteMonitor folds a recorded test or unsub into the in-memory roster the
// API serves, with the same rules as writeMonitor.
func (s *Store) noteMonitor(kind monitorKind, sender, hash, at string) {
	s.sosMu.Lock()
	defer s.sosMu.Unlock()
	m, ok := s.sosMonitors[sender]
	if !ok {
		m = &ChannelMonitor{Name: sender}
		s.sosMonitors[sender] = m
	}
	if kind == monitorUnsub {
		if at > m.UnsubAt {
			m.UnsubAt = at
		}
		return
	}
	if m.FirstTest == "" || at < m.FirstTest {
		m.FirstTest = at
	}
	if at > m.LastTest {
		m.LastTest = at
	}
	if s.sosLastHash[sender] != hash {
		m.Tests++
	}
	s.sosLastHash[sender] = hash
}

// loadSOSMonitors fills the in-memory roster from the table at open.
func (s *Store) loadSOSMonitors() error {
	rows, err := s.db.Query(`SELECT sender, first_test, last_test, tests, last_hash, unsub_at
		FROM channel_monitors WHERE channel = ?`, SOSChannel)
	if err != nil {
		return err
	}
	defer rows.Close()
	mons := map[string]*ChannelMonitor{}
	hashes := map[string]string{}
	for rows.Next() {
		var m ChannelMonitor
		var h string
		if err := rows.Scan(&m.Name, &m.FirstTest, &m.LastTest, &m.Tests, &h, &m.UnsubAt); err != nil {
			return err
		}
		mons[m.Name] = &m
		hashes[m.Name] = h
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.sosMu.Lock()
	s.sosMonitors, s.sosLastHash = mons, hashes
	s.sosMu.Unlock()
	return nil
}

// SOSMonitors returns the current #sos roster (unsubscribed senders left
// out), most recent test first. Served from memory — the endpoint never
// queues on the single DB connection.
func (s *Store) SOSMonitors() []ChannelMonitor {
	s.sosMu.RLock()
	out := make([]ChannelMonitor, 0, len(s.sosMonitors))
	for _, m := range s.sosMonitors {
		if m.active() {
			out = append(out, *m)
		}
	}
	s.sosMu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastTest != out[j].LastTest {
			return out[i].LastTest > out[j].LastTest
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// BackfillSOSMonitors scans the retained history once for #sos tests and unsubs,
// so monitors who checked in before this feature shipped are listed. Batched
// by hour of received_at (idx_obs_received), reading outside any transaction
// and writing each batch's finds in a short write-only one, releasing s.mu and
// the connection between batches — never one pass over observations (see
// [[ridgeline-database]]). Covers [now-days, now]; a no-op once marked done.
// RegisterSOSChannel must have run first, or nothing decrypts.
func (s *Store) BackfillSOSMonitors(ctx context.Context, days int, now time.Time) (scanned, found int, err error) {
	var done int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM channel_monitor_backfills WHERE channel = ?`,
		sosBackfillMarker).Scan(&done); err != nil {
		return 0, 0, err
	}
	if done > 0 {
		return 0, 0, nil
	}

	type hit struct {
		kind             monitorKind
		sender, hash, at string
	}
	// Copies of one transmission land within seconds of each other, so a
	// two-window seen set dedupes them without holding every hash in memory.
	seen, prevSeen := map[string]bool{}, map[string]bool{}
	end := now.UTC().Truncate(time.Hour).Add(time.Hour)
	for from := end.Add(-time.Duration(days) * 24 * time.Hour); from.Before(end); from = from.Add(time.Hour) {
		if err := ctx.Err(); err != nil {
			return scanned, found, err
		}
		to := from.Add(time.Hour)
		rows, err := s.db.Query(`
			SELECT message_hash, raw_hex, received_at FROM observations
			WHERE received_at >= ? AND received_at < ? AND payload_type = 'GroupText'`,
			from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano))
		if err != nil {
			return scanned, found, err
		}
		var hits []hit
		for rows.Next() {
			var h, raw, at string
			if err := rows.Scan(&h, &raw, &at); err != nil {
				rows.Close()
				return scanned, found, err
			}
			scanned++
			if seen[h] || prevSeen[h] {
				continue
			}
			seen[h] = true
			p, err := meshcore.DecodeHex(raw)
			if err != nil {
				continue
			}
			if sender, kind := sosMonitorMsg(p); kind != notMonitor {
				hits = append(hits, hit{kind, sender, h, at})
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return scanned, found, err
		}
		prevSeen, seen = seen, map[string]bool{}
		if len(hits) == 0 {
			continue
		}
		s.mu.Lock()
		tx, err := s.db.Begin()
		if err == nil {
			for _, h := range hits {
				if err = writeMonitor(tx, h.kind, SOSChannel, h.sender, h.hash, h.at); err != nil {
					break
				}
			}
			if err == nil {
				err = tx.Commit()
			} else {
				tx.Rollback()
			}
		}
		s.mu.Unlock()
		if err != nil {
			return scanned, found, fmt.Errorf("store: sos backfill write: %w", err)
		}
		found += len(hits)
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO channel_monitor_backfills (channel, done_at) VALUES (?,?)`,
		sosBackfillMarker, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return scanned, found, err
	}
	// Re-read so the roster carries the backfilled rows merged with anything
	// live ingest wrote meanwhile (the table is the merge point). Under s.mu so
	// no Record commits between the read and the swap and gets dropped.
	s.mu.Lock()
	defer s.mu.Unlock()
	return scanned, found, s.loadSOSMonitors()
}
