package store

import (
	_ "embed"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

//go:embed channel_wordlist.txt
var embeddedWordlist string

// ChannelCandidate is one row of the hashtag-channel discovery table.
type ChannelCandidate struct {
	Name        string `json:"name"`   // without leading '#'
	Source      string `json:"source"` // harvested | user | config
	Status      string `json:"status"` // pending | confirmed
	KeyHex      string `json:"keyHex,omitempty"`
	FirstAdded  string `json:"firstAdded"`
	ConfirmedAt string `json:"confirmedAt,omitempty"`
}

// Channel-candidate sources and statuses.
const (
	ChannelSourceHarvested = "harvested" // a #tag seen in a decrypted message
	ChannelSourceUser      = "user"      // submitted by a signed-in user
	ChannelSourceConfig    = "config"    // seeded from config
	ChannelSourceWordlist  = "wordlist"  // tried from the brute-force wordlist

	ChannelPending   = "pending"
	ChannelConfirmed = "confirmed"
)

// normalizeChannelName trims a submitted name to what the firmware actually
// hashes: no leading '#', no surrounding space, and at most 31 bytes (the on-air
// name field is 32 UTF-8 bytes including the '#'). Returns "" if nothing usable
// remains. Case is PRESERVED — the derivation is verbatim.
func normalizeChannelName(name string) string {
	n := strings.TrimSpace(name)
	n = strings.TrimPrefix(n, "#")
	n = strings.TrimSpace(n)
	if n == "" || len(n) > 31 {
		return ""
	}
	// A name with whitespace or a '#' inside is not a plausible hashtag channel.
	if strings.ContainsAny(n, " \t\r\n#") {
		return ""
	}
	return n
}

// AddChannelCandidate records a name to try, if not already present. It never
// downgrades a confirmed row or overwrites an earlier source — the first sighting
// wins, so a user submission does not relabel a channel we harvested, and a
// re-harvest does not revert a confirmation. Returns the normalized name stored,
// or "" if the input was not a usable hashtag name.
func (s *Store) AddChannelCandidate(name, source string, addedBy *int64) (string, error) {
	n := normalizeChannelName(name)
	if n == "" {
		return "", nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.db.Exec(`
		INSERT INTO channel_candidates (name, source, status, added_by, first_added)
		VALUES (?,?,?,?,?)
		ON CONFLICT(name) DO NOTHING`,
		n, source, ChannelPending, addedBy, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return "", err
	}
	if added, _ := r.RowsAffected(); added > 0 {
		s.chanMu.Lock()
		s.pendingChans++
		s.chanMu.Unlock()
	}
	return n, nil
}

// ConfirmChannel marks a candidate confirmed and records its key. Idempotent.
func (s *Store) ConfirmChannel(name, keyHex string) error {
	return s.confirmChannel(name, keyHex, ChannelSourceConfig)
}

// confirmChannel marks a candidate confirmed. source labels the row only when it
// is being INSERTed fresh (a name not previously a candidate); an existing row
// keeps its original source. An empty source defaults to config.
func (s *Store) confirmChannel(name, keyHex, source string) error {
	if source == "" {
		source = ChannelSourceConfig
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var wasPending bool
	_ = s.db.QueryRow(`SELECT status = 'pending' FROM channel_candidates WHERE name = ?`, name).Scan(&wasPending)
	_, err := s.db.Exec(`
		INSERT INTO channel_candidates (name, source, status, key_hex, first_added, confirmed_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(name) DO UPDATE SET
			status = 'confirmed',
			key_hex = excluded.key_hex,
			confirmed_at = COALESCE(channel_candidates.confirmed_at, excluded.confirmed_at)`,
		name, source, ChannelConfirmed, keyHex, now, now)
	if err != nil {
		return err
	}
	// Mirror the stored row (an existing candidate keeps its source and
	// first_added) into the in-memory list the discovery endpoint serves.
	c := ChannelCandidate{Status: ChannelConfirmed}
	if err := s.db.QueryRow(`
		SELECT name, source, COALESCE(key_hex,''), first_added, COALESCE(confirmed_at,'')
		FROM channel_candidates WHERE name = ?`, name).
		Scan(&c.Name, &c.Source, &c.KeyHex, &c.FirstAdded, &c.ConfirmedAt); err != nil {
		return err
	}
	s.chanMu.Lock()
	defer s.chanMu.Unlock()
	if wasPending && s.pendingChans > 0 {
		s.pendingChans--
	}
	for i := range s.confirmedChans {
		if s.confirmedChans[i].Name == c.Name {
			s.confirmedChans[i] = c
			return nil
		}
	}
	s.confirmedChans = append(s.confirmedChans, c)
	return nil
}

// CachedConfirmedChannels returns the confirmed channels and candidate counts
// from memory, without touching the database. Current as of the last
// confirmation; the pending count is refreshed each discovery pass.
func (s *Store) CachedConfirmedChannels() (chans []ChannelCandidate, pending, confirmed int) {
	s.chanMu.RLock()
	defer s.chanMu.RUnlock()
	chans = append([]ChannelCandidate{}, s.confirmedChans...)
	return chans, s.pendingChans, len(chans)
}

func (s *Store) setPendingChans(n int) {
	s.chanMu.Lock()
	s.pendingChans = n
	s.chanMu.Unlock()
}

// PendingChannelNames returns the names still awaiting confirmation.
func (s *Store) PendingChannelNames() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM channel_candidates WHERE status = 'pending'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ConfirmedChannels returns confirmed channels with their derived keys, for
// loading into the decoder at startup.
func (s *Store) ConfirmedChannels() ([]ChannelCandidate, error) {
	rows, err := s.db.Query(`
		SELECT name, source, COALESCE(key_hex,''), first_added, COALESCE(confirmed_at,'')
		FROM channel_candidates WHERE status = 'confirmed' ORDER BY confirmed_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChannelCandidate
	for rows.Next() {
		c := ChannelCandidate{Status: ChannelConfirmed}
		if err := rows.Scan(&c.Name, &c.Source, &c.KeyHex, &c.FirstAdded, &c.ConfirmedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountChannelCandidates returns (pending, confirmed) counts for status display.
func (s *Store) CountChannelCandidates() (pending, confirmed int, err error) {
	err = s.db.QueryRow(`
		SELECT
			COUNT(*) FILTER (WHERE status='pending'),
			COUNT(*) FILTER (WHERE status='confirmed')
		FROM channel_candidates`).Scan(&pending, &confirmed)
	return
}

// hashtagRef matches a #tag referenced in free text: '#' then name characters,
// stopping at whitespace or punctuation. Deliberately conservative (letters,
// digits, _, -) so it harvests real channel references, not every '#'. Names on
// the mesh can technically contain any byte, but those are not discoverable from
// a chat mention anyway — only an exact candidate confirms them.
var hashtagRef = regexp.MustCompile(`#([A-Za-z0-9_-]{1,31})`)

// groupTextWindow returns raw GroupText observations in [cutoff, now], newest
// first, capped. ⚠ ORDER BY received_at (indexed) — see the warning on rawSince
// and [[ridgeline-database]]: ordering this by id plans a full table SCAN.
func (s *Store) groupTextWindow(cutoff string, limit int) ([]string, error) {
	if limit <= 0 || limit > 50000 {
		limit = 20000
	}
	rows, err := s.db.Query(`
		SELECT raw_hex FROM observations
		WHERE received_at >= ? AND payload_type = 'GroupText'
		ORDER BY received_at DESC
		LIMIT ?`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// DiscoverChannels runs one discovery pass over the group-text window [since, now]:
//
//  1. Harvest #tags from messages that already decrypt (Public and any confirmed
//     channel) and record them as pending candidates.
//  2. For every pending candidate, derive SHA256("#"+name)[:16] and try to decrypt
//     the undecryptable group payloads whose channel-hash byte matches. A verifying
//     MAC confirms the name — it is proof, not a guess.
//
// A confirmed channel is persisted and registered with the decoder, so its past
// messages decrypt on the next analytics re-decode and its future ones live.
// Returns the names newly confirmed this pass.
func (s *Store) DiscoverChannels(since string, limit int) ([]string, error) {
	raws, err := s.groupTextWindow(since, limit)
	if err != nil {
		return nil, err
	}

	// One decode pass: harvest from what decrypts, bucket what does not by its
	// channel-hash byte so each candidate only tests its own bucket.
	buckets := make(map[byte][][]byte)
	harvested := map[string]bool{}
	for _, raw := range raws {
		p, err := meshcore.DecodeHex(raw)
		if err != nil || p == nil || p.GroupText == nil {
			continue
		}
		if p.GroupText.Decrypted {
			for _, m := range hashtagRef.FindAllStringSubmatch(p.GroupText.Message, -1) {
				harvested[m[1]] = true
			}
			continue
		}
		payload, err := hex.DecodeString(p.PayloadRaw)
		if err != nil || len(payload) < 3 {
			continue
		}
		buckets[payload[0]] = append(buckets[payload[0]], payload)
	}

	for name := range harvested {
		if _, err := s.AddChannelCandidate(name, ChannelSourceHarvested, nil); err != nil {
			return nil, err
		}
	}

	pending, err := s.PendingChannelNames()
	if err != nil {
		return nil, err
	}

	var confirmed []string
	got, err := s.matchNames(buckets, pending, "")
	if err != nil {
		return nil, err
	}
	confirmed = append(confirmed, got...)

	// Brute-force pass: try the wordlist transiently. These are NOT stored as
	// pending (that would bloat the table with thousands of never-confirmed
	// rows); only a hit is persisted. A wordlist name whose channel-hash byte
	// is not among the live buckets costs nothing, so a large list stays cheap.
	if len(s.channelWordlist) > 0 {
		got, err := s.matchNames(buckets, s.channelWordlist, ChannelSourceWordlist)
		if err != nil {
			return nil, err
		}
		confirmed = append(confirmed, got...)
	}
	if p, _, err := s.CountChannelCandidates(); err == nil {
		s.setPendingChans(p)
	}
	return confirmed, nil
}

// matchNames tries each name against the bucket of undecryptable payloads whose
// channel-hash byte it derives to, confirming and registering any that verify.
// source labels a newly-stored confirmation ("" lets ConfirmChannel keep its
// default). Skips names already registered with the decoder, so repeated passes
// and wordlist/pending overlap do no redundant work.
func (s *Store) matchNames(buckets map[byte][][]byte, names []string, source string) ([]string, error) {
	var confirmed []string
	for _, name := range names {
		key := meshcore.DeriveHashtagKey(name)
		bucket := buckets[meshcore.ChannelHashByte(key)]
		if len(bucket) == 0 {
			continue
		}
		for _, payload := range bucket {
			if _, _, _, ok := meshcore.VerifyGroupText(payload, key); ok {
				if err := s.confirmChannel(name, hex.EncodeToString(key), source); err != nil {
					return nil, err
				}
				meshcore.RegisterChannel(name, key)
				confirmed = append(confirmed, name)
				break
			}
		}
	}
	return confirmed, nil
}

// SetChannelWordlist loads brute-force candidate names (one per line, '#'
// optional, blanks and '#'-only lines ignored). Call once at startup.
func (s *Store) SetChannelWordlist(words []string) {
	seen := map[string]bool{}
	var out []string
	for _, w := range words {
		n := normalizeChannelName(w)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	s.channelWordlist = out
}

// DefaultChannelWordlist returns the built-in starter wordlist.
func DefaultChannelWordlist() []string {
	return strings.Split(strings.TrimSpace(embeddedWordlist), "\n")
}

// LoadConfirmedChannels registers every already-confirmed channel with the
// decoder and seeds the in-memory list the discovery endpoint serves. Called
// once at startup so historical messages decrypt immediately.
func (s *Store) LoadConfirmedChannels() (int, error) {
	chans, err := s.ConfirmedChannels()
	if err != nil {
		return 0, err
	}
	pending, _, err := s.CountChannelCandidates()
	if err != nil {
		return 0, err
	}
	s.chanMu.Lock()
	s.confirmedChans = append([]ChannelCandidate{}, chans...)
	s.pendingChans = pending
	s.chanMu.Unlock()
	n := 0
	for _, c := range chans {
		key, err := hex.DecodeString(c.KeyHex)
		if err != nil || len(key) != 16 {
			continue
		}
		meshcore.RegisterChannel(c.Name, key)
		n++
	}
	return n, nil
}
