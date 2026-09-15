package store

import (
	"testing"
	"time"
)

// The two presets this mesh accepts while it moves between channels.
var bothPresets = []string{"910.425,62.5,7,5", "909.000,62.5,7,5"}

func TestRadioGuardAcceptsRealObservers(t *testing.T) {
	s := testStore(t)
	s.SetAllowedRadios(bothPresets)

	// Every one of these is a real observer configuration seen on this mesh.
	// None of them may be quarantined.
	for _, tc := range []struct{ name, radio, why string }{
		{"typical", "910.425,62.5,7,5", "the base preset"},
		{"far side", "909.0,62.5,7,5", "909 written with one decimal — same channel"},
		{"coding rate 8", "910.425,62.5,7,8", "CR rides in the LoRa header; a receiver decodes any rate"},
		{"synthesised centre freq", "910.4249877,62.5,7,5", "a radio reporting its own synthesiser value"},
	} {
		if !s.radioAllowed(tc.radio) {
			t.Errorf("%s (%s): %q was refused — %s", tc.name, tc.radio, tc.radio, tc.why)
		}
	}
}

func TestRadioGuardRefusesForeignMesh(t *testing.T) {
	s := testStore(t)
	s.SetAllowedRadios(bothPresets)
	for _, r := range []string{
		"915.0,125,9,5",    // MeshCore's default North America preset
		"869.525,250,11,5", // EU
		"910.425,250,7,5",  // right channel, wrong bandwidth — cannot hear us
		"910.425,62.5,8,5", // right channel, wrong spreading factor
	} {
		if s.radioAllowed(r) {
			t.Errorf("%q should have been refused: it is a different network", r)
		}
	}
}

// Failing open matters more than failing safe here: a new observer publishes
// packets before its first status, and "we do not know yet" must not cost data.
func TestRadioGuardFailsOpenOnUnknown(t *testing.T) {
	s := testStore(t)
	s.SetAllowedRadios(bothPresets)
	for _, r := range []string{"", "   ", "garbage", "910.425", "910.425,62.5"} {
		if !s.radioAllowed(r) {
			t.Errorf("%q must be allowed: an unknown preset is not evidence of a wrong one", r)
		}
	}
}

func TestRadioGuardOffByDefault(t *testing.T) {
	s := testStore(t)
	if s.AllowedRadioCount() != 0 {
		t.Fatal("guard must be off until configured")
	}
	if !s.radioAllowed("915.0,125,9,5") {
		t.Error("with no presets configured nothing may be refused")
	}
}

// A config full of typos must not quarantine the whole network.
func TestRadioGuardIgnoresUnparseableConfig(t *testing.T) {
	s := testStore(t)
	s.SetAllowedRadios([]string{"nonsense", "", "910.425"})
	if s.AllowedRadioCount() != 0 {
		t.Fatalf("unparseable entries must be dropped, got %d", s.AllowedRadioCount())
	}
	if !s.radioAllowed("915.0,125,9,5") {
		t.Error("with nothing parseable the guard must stay off, not refuse everything")
	}
}

func TestRadioGuardQuarantineRoundTrip(t *testing.T) {
	s := testStore(t)
	s.SetAllowedRadios(bothPresets)
	const id = "obs-1"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(
		`INSERT INTO observers (id, first_seen, last_seen, packet_count) VALUES (?,?,?,0)`,
		id, now, now); err != nil {
		t.Fatal(err)
	}

	bad, changed := s.EvaluateObserverRadio(id, "915.0,125,9,5", now)
	if !bad || !changed {
		t.Fatalf("expected quarantine, got bad=%v changed=%v", bad, changed)
	}
	if !s.ObserverRadioQuarantined(id) {
		t.Fatal("observer should be quarantined")
	}
	// Idempotent: a second identical status is not a transition.
	if _, changed = s.EvaluateObserverRadio(id, "915.0,125,9,5", now); changed {
		t.Error("re-reporting the same bad preset must not count as a change")
	}
	// Survives a reload — the quarantine is in the table, not only in memory.
	if err := s.loadRadioQuarantine(); err != nil {
		t.Fatal(err)
	}
	if !s.ObserverRadioQuarantined(id) {
		t.Fatal("quarantine must survive a reload")
	}
	// Fixing the radio readmits without anyone intervening.
	bad, changed = s.EvaluateObserverRadio(id, "909.0,62.5,7,5", now)
	if bad || !changed {
		t.Fatalf("expected readmission, got bad=%v changed=%v", bad, changed)
	}
	if s.ObserverRadioQuarantined(id) {
		t.Fatal("observer should have been readmitted")
	}
}

// The trap standby fell into: a quarantined observer whose last_seen freezes is
// swept away by DeleteStaleObservers within the hour, taking the quarantine with
// it and letting the observer back in clean.
func TestRadioQuarantineKeepsLastSeenAdvancing(t *testing.T) {
	s := testStore(t)
	s.SetAllowedRadios(bothPresets)
	const id = "obs-2"
	old := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano)
	if _, err := s.db.Exec(
		`INSERT INTO observers (id, first_seen, last_seen, packet_count) VALUES (?,?,?,0)`,
		id, old, old); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	s.RecordRadioQuarantineDrop(id, now)

	var got string
	if err := s.db.QueryRow(`SELECT last_seen FROM observers WHERE id = ?`, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got == old {
		t.Fatal("last_seen did not advance — retention would sweep this observer away")
	}
	if n := s.RadioQuarantineDropped()[id]; n != 1 {
		t.Errorf("dropped count = %d, want 1", n)
	}
}

// A publisher that streams packets but never reports a radio must not cycle
// invisibly: held, expired, discarded, held again, forever, with no observer
// row anywhere because only storing a packet or receiving a status creates one.
func TestSilentObserverIsQuarantinedAndVisible(t *testing.T) {
	s := testStore(t)
	s.SetAllowedRadios(bothPresets)
	const id = "silent-1"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	// No row exists yet — that is the whole problem being fixed.
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM observers WHERE id = ?`, id).Scan(&n)
	if n != 0 {
		t.Fatalf("precondition: expected no observer row, got %d", n)
	}

	if err := s.QuarantineSilentObserver(id, "Mystery Publisher", now); err != nil {
		t.Fatal(err)
	}
	if !s.ObserverRadioQuarantined(id) {
		t.Fatal("silent publisher should be quarantined")
	}

	var name, reason, at string
	if err := s.db.QueryRow(
		`SELECT COALESCE(name,''), COALESCE(radio_quarantine_reason,''), COALESCE(radio_quarantined_at,'')
		   FROM observers WHERE id = ?`, id).Scan(&name, &reason, &at); err != nil {
		t.Fatalf("no observer row created — it would stay invisible: %v", err)
	}
	if name != "Mystery Publisher" || reason != "no-status" || at == "" {
		t.Errorf("row = name %q reason %q at %q", name, reason, at)
	}

	// Idempotent: the sweep must not churn the row on every pass.
	if err := s.QuarantineSilentObserver(id, "Mystery Publisher", now); err != nil {
		t.Fatal(err)
	}

	// And it recovers by itself the moment it finally identifies correctly.
	bad, changed := s.EvaluateObserverRadio(id, "910.425,62.5,7,5", now)
	if bad || !changed {
		t.Fatalf("a late but valid status must readmit: bad=%v changed=%v", bad, changed)
	}
	var left string
	s.db.QueryRow(`SELECT COALESCE(radio_quarantine_reason,'') FROM observers WHERE id = ?`, id).Scan(&left)
	if left != "" {
		t.Errorf("quarantine reason not cleared on readmission: %q", left)
	}
}

// The guard being off must not create rows for publishers it is not judging.
func TestSilentQuarantineNoopWhenGuardOff(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.QuarantineSilentObserver("x", "X", now); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM observers WHERE id = 'x'`).Scan(&n)
	if n != 0 {
		t.Error("with the guard off, nothing should be quarantined or created")
	}
}
