package store

import (
	"strings"
	"time"

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

// EvaluateObserverRadio applies the guard to a freshly reported preset and
// returns whether the observer is now quarantined, and whether that changed.
//
// Called from the status path, which is the only place a preset arrives.
// Clearing is as important as setting: an operator who fixes their radio must
// come back automatically, without anyone noticing and intervening.
func (s *Store) EvaluateObserverRadio(observerID, reported, at string) (quarantined, changed bool) {
	if observerID == "" || s.AllowedRadioCount() == 0 {
		return false, false
	}
	bad := !s.radioAllowed(reported)

	s.radioMu.Lock()
	was := s.quarantinedRadios[observerID]
	firstVerdict := !s.confirmedRadios[observerID] && !was
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

	// A first verdict is a change even when it is "accepted": it is what
	// releases anything the holding pen is keeping for this observer.
	if bad == was && !firstVerdict {
		return bad, false
	}

	s.mu.Lock()
	if bad {
		s.db.Exec(`UPDATE observers SET radio_quarantined_at = ?, radio_quarantine_radio = ? WHERE id = ?`,
			at, radiopkg.Normalize(strings.TrimSpace(reported)), observerID)
	} else {
		s.db.Exec(`UPDATE observers SET radio_quarantined_at = NULL, radio_quarantine_radio = NULL WHERE id = ?`,
			observerID)
	}
	s.mu.Unlock()
	return bad, true
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
