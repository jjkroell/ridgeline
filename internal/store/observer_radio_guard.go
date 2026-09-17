package store

import (
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	radiopkg "github.com/jjkroell/ridgeline/internal/radio"
)

// Radio-preset guard.
//
// An observer reporting a preset this mesh does not run is hearing a DIFFERENT
// network. Its packets arrive over MQTT indistinguishable from this mesh's own —
// same shape, same fields, plausible node keys — and once stored there is nothing
// downstream that can separate them. They invent nodes that are not on the mesh
// and links that do not exist, and they do it quietly.
//
// So the check is at ingest, not at connect. The radio config is not known when
// the broker authenticates: it arrives later, in a /status message. The observer
// is allowed to connect, its first status is evaluated, and from then on its
// packets are either kept or dropped.
//
// What this deliberately does NOT do:
//
//   - It never refuses the MQTT connection. A quarantined observer stays
//     connected and keeps publishing status, which is the only way the operator
//     can see what it is actually set to — and therefore the only way to tell
//     its owner what to fix. Denying at the broker leaves them with "auth
//     failed" and no idea why.
//   - It never quarantines an observer whose preset is unknown or unparseable.
//     A new observer publishes packets before its first status, and "probably
//     wrong" is not a reason to throw away data.
//   - It ignores coding rate, via radio.SameSegment. LoRa carries CR in the
//     packet header, so a receiver decodes any sender's rate whatever its own
//     is set to. An observer on CR8 hears this mesh perfectly.

// SetAllowedRadios installs the presets an observer may report. Empty turns the
// check off, which is the default — the presets belong to one deployment's mesh.
//
// Unparseable entries are dropped rather than treated as "match nothing": a
// typo in config must not quietly quarantine every observer on the network.
func (s *Store) SetAllowedRadios(list []string) {
	var ps []radiopkg.Profile
	for _, e := range list {
		p := radiopkg.Parse(e)
		if p.HasFreq && p.HasBW && p.HasSF {
			ps = append(ps, p)
		}
	}
	s.radioMu.Lock()
	s.allowedRadios = ps
	s.radioMu.Unlock()

	// Re-vet against the list just installed. Open() rebuilt the confirmed set
	// before any list existed, and with nothing to check against radioAllowed
	// says yes to everything — so every observer with a stored preset came up
	// "confirmed", foreign ones included, and stayed that way until its next
	// status: a five-minute window of stored foreign traffic on every restart,
	// and on the day the list is narrowed, the whole fleet at once.
	if err := s.loadRadioQuarantine(); err != nil {
		return
	}
	s.backfillRadioOK()
}

// backfillRadioOK gives an observer confirmed from its stored preset the anchor
// a live status would have set. Its last status IS the one that passed —
// that is what confirmed it — so radio_ok_at = last_status_at is exact, not a
// guess. Without it a pre-existing observer that later retunes would be
// quarantined but have nothing retracted.
func (s *Store) backfillRadioOK() {
	s.radioMu.RLock()
	ids := make([]string, 0, len(s.confirmedRadios))
	for id := range s.confirmedRadios {
		ids = append(ids, id)
	}
	s.radioMu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		s.db.Exec(`UPDATE observers SET radio_ok_at = last_status_at
		           WHERE id = ? AND radio_ok_at IS NULL AND last_status_at IS NOT NULL`, id)
	}
}

// AllowedRadioCount reports how many presets are configured; 0 means the guard
// is inactive.
func (s *Store) AllowedRadioCount() int {
	s.radioMu.RLock()
	defer s.radioMu.RUnlock()
	return len(s.allowedRadios)
}

// radioAllowed reports whether a reported preset is one this mesh accepts.
// An unknown preset is allowed: see the note above on failing open.
func (s *Store) radioAllowed(reported string) bool {
	s.radioMu.RLock()
	allowed := s.allowedRadios
	s.radioMu.RUnlock()
	if len(allowed) == 0 {
		return true
	}
	p := radiopkg.Parse(reported)
	if !p.HasFreq || !p.HasBW || !p.HasSF {
		return true
	}
	for _, a := range allowed {
		if p.SameSegment(a) {
			return true
		}
	}
	return false
}

// ObserverRadioConfirmed reports whether this observer has reported a preset
// that passed. It is NOT the negation of quarantined: an observer that has never
// sent a status is neither, and that third state is what the holding pen exists
// for — see Ingestor.holdObservation.
func (s *Store) ObserverRadioConfirmed(observerID string) bool {
	if observerID == "" {
		return false
	}
	s.radioMu.RLock()
	defer s.radioMu.RUnlock()
	return s.confirmedRadios[observerID]
}

// loadRadioQuarantine rebuilds the verdict sets from the table.
//
// Confirmed is rebuilt from the stored radio column rather than persisted
// separately: an observer that was accepted before a restart must not be held
// again, and its last reported preset is the evidence that it passed.
func (s *Store) loadRadioQuarantine() error {
	rows, err := s.db.Query(
		`SELECT id FROM observers WHERE radio_quarantined_at IS NOT NULL AND radio_quarantined_at <> ''`)
	if err != nil {
		return err
	}
	defer rows.Close()
	set := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		set[id] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Anything with a stored preset that passes has already been vetted.
	ok := map[string]bool{}
	crows, err := s.db.Query(`SELECT id, COALESCE(radio,'') FROM observers WHERE COALESCE(radio,'') <> ''`)
	if err != nil {
		return err
	}
	defer crows.Close()
	for crows.Next() {
		var id, r string
		if err := crows.Scan(&id, &r); err != nil {
			return err
		}
		if !set[id] && s.radioAllowed(r) {
			ok[id] = true
		}
	}
	if err := crows.Err(); err != nil {
		return err
	}

	s.radioMu.Lock()
	s.quarantinedRadios = set
	s.confirmedRadios = ok
	s.radioMu.Unlock()
	return nil
}

// ObserverRadioQuarantined reports whether this observer's packets are being
// dropped for reporting a preset off this mesh.
func (s *Store) ObserverRadioQuarantined(observerID string) bool {
	if observerID == "" {
		return false
	}
	s.radioMu.RLock()
	defer s.radioMu.RUnlock()
	return s.quarantinedRadios[observerID]
}

// RadioVerdict is the outcome of evaluating one reported preset.
type RadioVerdict struct {
	Quarantined bool
	Changed     bool // a transition, in either direction; the only thing worth logging
	// VouchedSince is set only on a confirmed → quarantined transition: the
	// time of the last status that passed. Everything this observer stored
	// after it was accepted on the strength of a preset it has since left, and
	// the caller is expected to retract it — see RetractObserverSince.
	VouchedSince string
}

// EvaluateObserverRadio applies the guard to a freshly reported preset.
//
// Called from the status path, which is the only place a preset arrives.
// Clearing is as important as setting: an operator who fixes their radio must
// come back automatically, without anyone noticing and intervening.
//
// retained says the status is the broker's replay of the observer's last one,
// not a live report. It still decides admission — that is what readmits an
// observer fixed while the daemon was away — but it must not move the anchor
// forward: it proves nothing about the observer NOW, and a later retraction
// would then start too late and leave foreign rows from before the reconnect.
// It only sets the anchor when there is none, because no anchor means no
// retraction at all, which is worse.
func (s *Store) EvaluateObserverRadio(observerID, reported, at string, retained bool) RadioVerdict {
	if observerID == "" || s.AllowedRadioCount() == 0 {
		return RadioVerdict{}
	}
	bad := !s.radioAllowed(reported)

	s.radioMu.Lock()
	was := s.quarantinedRadios[observerID]
	wasConfirmed := s.confirmedRadios[observerID]
	firstVerdict := !wasConfirmed && !was
	if s.quarantinedRadios == nil {
		s.quarantinedRadios = map[string]bool{}
	}
	if s.confirmedRadios == nil {
		s.confirmedRadios = map[string]bool{}
	}
	if bad {
		s.quarantinedRadios[observerID] = true
		delete(s.confirmedRadios, observerID)
	} else {
		delete(s.quarantinedRadios, observerID)
		s.confirmedRadios[observerID] = true
	}
	s.radioMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// A first verdict is a change even when it is "accepted": it is what
	// releases anything the holding pen is keeping for this observer.
	if bad == was && !firstVerdict {
		if !bad {
			// Still good: move the anchor forward. This runs on every accepted
			// status, so a later fall from grace retracts one status cycle,
			// not everything since the observer first appeared.
			s.db.Exec(`UPDATE observers SET `+anchorSet(retained)+` WHERE id = ?`, at, observerID)
		}
		return RadioVerdict{Quarantined: bad}
	}

	v := RadioVerdict{Quarantined: bad, Changed: true}
	if bad {
		// Read the anchor before clearing it: it is the retraction's start, and
		// clearing it is what makes a second quarantine — after a readmission
		// that set a fresh one — retract from the fresh one, never from here.
		if wasConfirmed {
			var since *string
			s.db.QueryRow(`SELECT radio_ok_at FROM observers WHERE id = ?`, observerID).Scan(&since)
			if since != nil {
				v.VouchedSince = *since
			}
		}
		s.db.Exec(`UPDATE observers SET radio_quarantined_at = ?, radio_quarantine_radio = ?, radio_quarantine_reason = 'preset', radio_ok_at = NULL WHERE id = ?`,
			at, radiopkg.Normalize(strings.TrimSpace(reported)), observerID)
	} else {
		s.db.Exec(`UPDATE observers SET radio_quarantined_at = NULL, radio_quarantine_radio = NULL, radio_quarantine_reason = NULL, `+anchorSet(retained)+` WHERE id = ?`,
			at, observerID)
	}
	return v
}

// anchorSet is the SET clause that records an accepted status as the
// retraction anchor: the status time when live; when retained, whatever is
// already there, falling back to the status time only if nothing is.
func anchorSet(retained bool) string {
	if retained {
		return "radio_ok_at = COALESCE(radio_ok_at, ?)"
	}
	return "radio_ok_at = ?"
}

// radioTouchInterval throttles the last_seen refresh on the drop path, matching
// standbyTouchInterval — freshness is read in minutes, retention in hours.
const radioTouchInterval = time.Minute

// RecordRadioQuarantineDrop counts a dropped packet and keeps last_seen current.
//
// ⚠ last_seen MUST keep advancing. We really did hear from the observer — we
// just refused what it said. A frozen last_seen makes it read as "Silent" within
// minutes and, far worse, DeleteStaleObservers sweeps the row away after an
// hour, taking the quarantine with it and letting the observer back in clean.
// This is the same trap observer standby hit; see RecordStandbyDrop.
func (s *Store) RecordRadioQuarantineDrop(observerID, at string) {
	s.radioMu.Lock()
	if s.radioDropped == nil {
		s.radioDropped = map[string]int64{}
	}
	s.radioDropped[observerID]++
	if s.radioSeen == nil {
		s.radioSeen = map[string]time.Time{}
	}
	now := time.Now()
	touch := now.Sub(s.radioSeen[observerID]) >= radioTouchInterval
	if touch {
		s.radioSeen[observerID] = now
	}
	s.radioMu.Unlock()

	if !touch {
		return
	}
	// Taken after releasing radioMu: never hold the hot-path lock while waiting
	// on the write lock.
	s.mu.Lock()
	s.db.Exec(`UPDATE observers SET last_seen = ? WHERE id = ?`, at, observerID)
	s.mu.Unlock()
}

// RadioQuarantineDropped returns packets dropped per observer since this daemon
// started — a live signal that the guard is working, not an audited total.
func (s *Store) RadioQuarantineDropped() map[string]int64 {
	s.radioMu.RLock()
	defer s.radioMu.RUnlock()
	out := make(map[string]int64, len(s.radioDropped))
	for k, v := range s.radioDropped {
		out[k] = v
	}
	return out
}

// QuarantineSilentObserver refuses an observer that publishes packets but never
// says what radio it is on.
//
// Without this such a publisher cycles forever: its packets are held, the pen
// expires, they are discarded, the next packet opens a fresh pen. Nothing is
// ever stored and memory stays bounded, but it never resolves — and because the
// observer row is only created by storing a packet or by receiving a status, it
// reached neither, so nothing about it was visible anywhere.
//
// So the row is created HERE, which is the first point at which there is
// something worth saying about this publisher: it has been talking for the whole
// grace period and has still not identified itself. From now on its packets are
// dropped at ingest rather than held, which is also cheaper.
//
// Creating a row for an unverified publisher is a deliberate trade. It is
// bounded in practice because reaching the broker at all requires a JWT signed
// by the publisher's own node key — this is not an open relay — and in time
// because one row can only appear per publisher per grace period.
//
// Recovery is automatic and needs no operator action: if it ever does send a
// good status, EvaluateObserverRadio clears the quarantine like any other.
func (s *Store) QuarantineSilentObserver(observerID, observerName, at string) error {
	if observerID == "" || s.AllowedRadioCount() == 0 {
		return nil
	}
	s.radioMu.Lock()
	if s.quarantinedRadios == nil {
		s.quarantinedRadios = map[string]bool{}
	}
	already := s.quarantinedRadios[observerID]
	s.quarantinedRadios[observerID] = true
	delete(s.confirmedRadios, observerID)
	s.radioMu.Unlock()
	if already {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO observers (id, name, first_seen, last_seen, packet_count,
		                       radio_quarantined_at, radio_quarantine_reason)
		VALUES (?,?,?,?,0,?, 'no-status')
		ON CONFLICT(id) DO UPDATE SET
			last_seen               = excluded.last_seen,
			name                    = COALESCE(NULLIF(excluded.name,''), observers.name),
			radio_quarantined_at    = excluded.radio_quarantined_at,
			radio_quarantine_reason = 'no-status'`,
		observerID, nullStr(observerName), at, at, at)
	return err
}

// RetractResult is what a retraction removed.
type RetractResult struct {
	Observations int64
	Nodes        int64
	// SkippedClaimed lists node keys that lost their only witness but were
	// kept because a user has claimed, annotated or located them.
	SkippedClaimed []string
}

// RetractObserverSince removes what an observer stored after `since` — the
// last status that passed — because its next status did not.
//
// This is the window the guard cannot close on its own. The verdict rides on
// /status, and status lands every five minutes; an observer that retunes to
// a foreign preset and keeps publishing is *confirmed* for the whole of that
// interval, and everything it hears on the other network goes straight to the
// database. Measured on prod: 10–30 packets per observer per cycle, 60 for the
// busiest, peaks near 200. Those rows invent nodes that are not on this mesh
// and links that do not exist, and nothing downstream can tell them apart.
//
// We cannot know when in the interval the retune happened, so the whole
// interval goes. Rows heard before `since` were vouched for by a status that
// passed and are untouched.
//
// Nodes: a node row is created only by a signature-valid advert, so the keys
// carried by the retracted rows are the nodes this window may have invented.
// One is deleted only if it first appeared inside the window AND no other
// observer heard an advert for it there — either says it was on this mesh
// before the retune, or someone on this mesh heard it too. That bounds both
// scans to the window: the only rows that can witness a node younger than
// `since` are rows younger than `since`. A node someone has claimed,
// annotated or located is kept and reported, as the observer-delete orphan
// pass does; their data is about a real radio.
//
// Not undone: a pre-existing node whose last_seen or advert_count a window
// advert bumped. That needs the same key to be heard on two networks, which
// is the same device, which is not a phantom.
func (s *Store) RetractObserverSince(observerID, since string) (RetractResult, error) {
	var res RetractResult
	if observerID == "" || since == "" {
		return res, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	// The rows being retracted, and the advert keys they carry.
	rows, err := tx.Query(
		`SELECT id, raw_hex FROM observations WHERE observer_id = ? AND received_at >= ?`,
		observerID, since)
	if err != nil {
		return res, err
	}
	var delIDs []int64
	condemned := map[string]bool{}
	for rows.Next() {
		var id int64
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return res, err
		}
		delIDs = append(delIDs, id)
		if k := advertKeyOf(raw); k != "" {
			condemned[k] = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}
	if len(delIDs) == 0 {
		return res, nil
	}

	// What everyone else heard in the same window.
	witnessed := map[string]bool{}
	if len(condemned) > 0 {
		wrows, err := tx.Query(
			`SELECT raw_hex FROM observations WHERE received_at >= ? AND COALESCE(observer_id,'') <> ?`,
			since, observerID)
		if err != nil {
			return res, err
		}
		for wrows.Next() {
			var raw string
			if err := wrows.Scan(&raw); err != nil {
				wrows.Close()
				return res, err
			}
			if k := advertKeyOf(raw); k != "" {
				witnessed[k] = true
			}
		}
		wrows.Close()
		if err := wrows.Err(); err != nil {
			return res, err
		}
	}

	for _, id := range delIDs {
		r, err := tx.Exec(`DELETE FROM observations WHERE id = ?`, id)
		if err != nil {
			return res, err
		}
		n, _ := r.RowsAffected()
		res.Observations += n
	}
	// packet_count is a running total bumped per stored row; keep it honest.
	if _, err := tx.Exec(
		`UPDATE observers SET packet_count = MAX(0, packet_count - ?) WHERE id = ?`,
		res.Observations, observerID); err != nil {
		return res, err
	}

	orphans := make([]string, 0, len(condemned))
	for k := range condemned {
		if !witnessed[k] {
			orphans = append(orphans, k)
		}
	}
	sort.Strings(orphans)
	for _, k := range orphans {
		var firstSeen string
		err := tx.QueryRow(`SELECT first_seen FROM nodes WHERE UPPER(pubkey) = ?`, k).Scan(&firstSeen)
		if err == sql.ErrNoRows {
			continue // the advert failed signature and never made a node
		}
		if err != nil {
			return res, err
		}
		if firstSeen < since {
			continue // known before the window: on this mesh, not invented here
		}
		var held bool
		if err := tx.QueryRow(`SELECT EXISTS(
			SELECT 1 FROM node_claims WHERE UPPER(node_pubkey) = ?
			UNION ALL SELECT 1 FROM node_notes WHERE UPPER(node_pubkey) = ?
			UNION ALL SELECT 1 FROM node_private_locations WHERE UPPER(node_pubkey) = ?
			UNION ALL SELECT 1 FROM location_shares WHERE UPPER(node_pubkey) = ?)`,
			k, k, k, k).Scan(&held); err != nil {
			return res, err
		}
		if held {
			res.SkippedClaimed = append(res.SkippedClaimed, k)
			continue
		}
		r, err := tx.Exec(`DELETE FROM nodes WHERE UPPER(pubkey) = ?`, k)
		if err != nil {
			return res, err
		}
		n, _ := r.RowsAffected()
		res.Nodes += n
	}

	return res, tx.Commit()
}

// advertKeyOf returns the uppercase advert pubkey a stored packet carries, or
// "" when it is not an advert (or does not decode).
func advertKeyOf(rawHex string) string {
	pkt, err := meshcore.DecodeHex(rawHex)
	if err != nil || pkt == nil || pkt.Advert == nil {
		return ""
	}
	return strings.ToUpper(pkt.Advert.PublicKey)
}
