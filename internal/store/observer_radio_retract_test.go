package store

import (
	"testing"
	"time"
)

// statusAt does what the ingest status path does: store the status, then judge
// the preset it carries.
func statusAt(t *testing.T, st *Store, id, radio string, at time.Time) RadioVerdict {
	t.Helper()
	ts := at.UTC().Format(time.RFC3339)
	if err := st.UpsertObserverStatus(id, id, "YVR", "", "{}", radio, ts); err != nil {
		t.Fatalf("status for %s: %v", id, err)
	}
	return st.EvaluateObserverRadio(id, radio, ts, false)
}

func radioOKAt(t *testing.T, st *Store, id string) string {
	t.Helper()
	var v string
	st.db.QueryRow(`SELECT COALESCE(radio_ok_at,'') FROM observers WHERE id = ?`, id).Scan(&v)
	return v
}

func obsCount(t *testing.T, st *Store, id string) int {
	t.Helper()
	var n int
	st.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE observer_id = ?`, id).Scan(&n)
	return n
}

// The hole the retraction closes: a confirmed observer retunes to another
// network and keeps feeding. Until its next status says so — one cycle, ~5
// minutes — its packets are stored, and every advert in them invents a node
// that is not on this mesh. The quarantine that finally lands must take that
// window back, including the node.
func TestRetuneRetractsTheWindow(t *testing.T) {
	st := testStore(t)
	st.SetAllowedRadios(bothPresets)
	const obs = "obs-A"
	good := time.Now().Add(-10 * time.Minute)

	if v := statusAt(t, st, obs, "910.425,62.5,7,5", good); v.Quarantined || !v.Changed {
		t.Fatalf("first good status: %+v", v)
	}
	// The retune happens somewhere in here; the observer keeps publishing.
	node := heardBy(t, st, obs)
	if !nodeExists(t, st, node) {
		t.Fatal("fixture advert did not create the node row")
	}

	v := statusAt(t, st, obs, "906.875,250,11,5", time.Now())
	if !v.Quarantined || !v.Changed {
		t.Fatalf("expected quarantine: %+v", v)
	}
	if want := good.UTC().Format(time.RFC3339); v.VouchedSince != want {
		t.Fatalf("VouchedSince = %q, want the last good status %q", v.VouchedSince, want)
	}
	res, err := st.RetractObserverSince(obs, v.VouchedSince)
	if err != nil {
		t.Fatal(err)
	}
	if res.Observations != 1 || res.Nodes != 1 {
		t.Errorf("retracted observations=%d nodes=%d, want 1 and 1", res.Observations, res.Nodes)
	}
	if nodeExists(t, st, node) {
		t.Error("the node the foreign window invented survived")
	}
	if n := obsCount(t, st, obs); n != 0 {
		t.Errorf("%d observations left, want 0", n)
	}
	var pc int
	st.db.QueryRow(`SELECT packet_count FROM observers WHERE id = ?`, obs).Scan(&pc)
	if pc != 0 {
		t.Errorf("packet_count = %d after retraction, want 0", pc)
	}
	if got := radioOKAt(t, st, obs); got != "" {
		t.Errorf("radio_ok_at = %q after quarantine, want cleared", got)
	}
}

// A node another observer also heard in the window was on this mesh: the
// retuned observer's copy goes, the node stays.
func TestRetractKeepsNodesWitnessedElsewhere(t *testing.T) {
	st := testStore(t)
	st.SetAllowedRadios(bothPresets)
	good := time.Now().Add(-10 * time.Minute)
	statusAt(t, st, "obs-A", "910.425,62.5,7,5", good)
	statusAt(t, st, "obs-B", "910.425,62.5,7,5", good)

	node := heardBy(t, st, "obs-A")
	heardBy(t, st, "obs-B")

	v := statusAt(t, st, "obs-A", "906.875,250,11,5", time.Now())
	res, err := st.RetractObserverSince("obs-A", v.VouchedSince)
	if err != nil {
		t.Fatal(err)
	}
	if res.Observations != 1 || res.Nodes != 0 {
		t.Errorf("retracted observations=%d nodes=%d, want 1 and 0", res.Observations, res.Nodes)
	}
	if !nodeExists(t, st, node) {
		t.Error("a node heard by a second observer was deleted")
	}
	if n := obsCount(t, st, "obs-B"); n != 1 {
		t.Errorf("obs-B lost rows: %d left, want 1", n)
	}
}

// A node known before the window was on this mesh before the retune. Its
// window copy goes; the node — and everything it stored before the last good
// status — stays.
func TestRetractKeepsPreExistingNodes(t *testing.T) {
	st := testStore(t)
	st.SetAllowedRadios(bothPresets)
	const obs = "obs-A"

	statusAt(t, st, obs, "910.425,62.5,7,5", time.Now().Add(-20*time.Minute))
	node := heardBy(t, st, obs) // before the last good status
	// Push first_seen and that row clearly before the anchor.
	old := time.Now().Add(-15 * time.Minute).UTC().Format(time.RFC3339Nano)
	st.db.Exec(`UPDATE nodes SET first_seen = ? WHERE UPPER(pubkey) = UPPER(?)`, old, node)
	st.db.Exec(`UPDATE observations SET received_at = ? WHERE observer_id = ?`, old, obs)

	good := time.Now().Add(-10 * time.Minute)
	if v := statusAt(t, st, obs, "910.425,62.5,7,5", good); v.Changed {
		t.Fatalf("a repeat good status is not a transition: %+v", v)
	}
	if got := radioOKAt(t, st, obs); got != good.UTC().Format(time.RFC3339) {
		t.Fatalf("radio_ok_at = %q, want advanced to the latest good status", got)
	}
	heardBy(t, st, obs) // the same node again, inside the window

	v := statusAt(t, st, obs, "906.875,250,11,5", time.Now())
	res, err := st.RetractObserverSince(obs, v.VouchedSince)
	if err != nil {
		t.Fatal(err)
	}
	if res.Observations != 1 || res.Nodes != 0 {
		t.Errorf("retracted observations=%d nodes=%d, want 1 and 0", res.Observations, res.Nodes)
	}
	if !nodeExists(t, st, node) {
		t.Error("a node known before the window was deleted")
	}
	if n := obsCount(t, st, obs); n != 1 {
		t.Errorf("%d observations left, want the pre-window one", n)
	}
}

// A user's claim outranks the inference, exactly as in the observer-delete
// orphan pass: the row goes, the node is kept and reported.
func TestRetractKeepsClaimedNodes(t *testing.T) {
	st := testStore(t)
	st.SetAllowedRadios(bothPresets)
	const obs = "obs-A"
	statusAt(t, st, obs, "910.425,62.5,7,5", time.Now().Add(-10*time.Minute))
	node := heardBy(t, st, obs)
	st.CreateUser("owner@example.com", "h", "Owner")
	u, _ := st.CreateUser("claimer@example.com", "h", "Claimer")
	if _, err := st.CreateVerifiedClaim(node, u.ID); err != nil {
		t.Fatal(err)
	}

	v := statusAt(t, st, obs, "906.875,250,11,5", time.Now())
	res, err := st.RetractObserverSince(obs, v.VouchedSince)
	if err != nil {
		t.Fatal(err)
	}
	if res.Nodes != 0 || len(res.SkippedClaimed) != 1 {
		t.Errorf("nodes=%d skipped=%v, want 0 and the claimed key", res.Nodes, res.SkippedClaimed)
	}
	if !nodeExists(t, st, node) {
		t.Error("a claimed node was deleted")
	}
}

// After readmission the anchor is fresh, so a second fall retracts only what
// came after the fix — never back to the first good status.
func TestRetractAnchorResetsOnReadmission(t *testing.T) {
	st := testStore(t)
	st.SetAllowedRadios(bothPresets)
	const obs = "obs-A"
	statusAt(t, st, obs, "910.425,62.5,7,5", time.Now().Add(-30*time.Minute))
	statusAt(t, st, obs, "906.875,250,11,5", time.Now().Add(-20*time.Minute)) // quarantined
	fixed := time.Now().Add(-10 * time.Minute)
	if v := statusAt(t, st, obs, "910.425,62.5,7,5", fixed); v.Quarantined || !v.Changed {
		t.Fatalf("readmission: %+v", v)
	}
	v := statusAt(t, st, obs, "906.875,250,11,5", time.Now())
	if want := fixed.UTC().Format(time.RFC3339); v.VouchedSince != want {
		t.Errorf("VouchedSince = %q, want the readmission %q", v.VouchedSince, want)
	}
}

// A first verdict that is bad has nothing to retract: nothing was stored, it
// was held, and the pen discards it.
func TestNoRetractionWithoutAVouch(t *testing.T) {
	st := testStore(t)
	st.SetAllowedRadios(bothPresets)
	if v := statusAt(t, st, "obs-A", "906.875,250,11,5", time.Now()); v.VouchedSince != "" {
		t.Errorf("VouchedSince = %q on a first bad verdict, want empty", v.VouchedSince)
	}
}

// Open() rebuilds the confirmed set before the allow list exists, and with no
// list radioAllowed says yes to everything. Installing the list must re-vet:
// an observer whose stored preset is foreign is NOT confirmed (it is held until
// its next status), and one whose stored preset passes gets the anchor a live
// status would have given it.
func TestSetAllowedRadiosReVetsStoredPresets(t *testing.T) {
	st := testStore(t)
	at := time.Now().Add(-3 * time.Minute).UTC().Format(time.RFC3339)
	for id, radio := range map[string]string{"obs-good": "910.425,62.5,7,5", "obs-foreign": "906.875,250,11,5"} {
		if err := st.UpsertObserverStatus(id, id, "YVR", "", "{}", radio, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.loadRadioQuarantine(); err != nil { // what Open() did
		t.Fatal(err)
	}
	if !st.ObserverRadioConfirmed("obs-foreign") {
		t.Fatal("precondition: with no list, everything reads confirmed")
	}

	st.SetAllowedRadios(bothPresets)
	if st.ObserverRadioConfirmed("obs-foreign") {
		t.Error("a foreign stored preset stayed confirmed after the list was installed")
	}
	if st.ObserverRadioQuarantined("obs-foreign") {
		t.Error("re-vetting must not quarantine on a stored value; that is the next status's call")
	}
	if !st.ObserverRadioConfirmed("obs-good") {
		t.Error("a passing stored preset lost its confirmation")
	}
	if got := radioOKAt(t, st, "obs-good"); got != at {
		t.Errorf("radio_ok_at = %q, want backfilled from last_status_at %q", got, at)
	}
	if got := radioOKAt(t, st, "obs-foreign"); got != "" {
		t.Errorf("radio_ok_at = %q for a foreign preset, want none — we never vouched for it", got)
	}
}

// A retained status is the broker replaying the observer's last report on
// reconnect. It readmits, but it must not move the anchor: it says nothing
// about the observer now, and a later retraction would start too late.
func TestRetainedStatusDoesNotAdvanceAnchor(t *testing.T) {
	st := testStore(t)
	st.SetAllowedRadios(bothPresets)
	const obs = "obs-A"
	live := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	if err := st.UpsertObserverStatus(obs, obs, "YVR", "", "{}", "910.425,62.5,7,5", live); err != nil {
		t.Fatal(err)
	}
	st.EvaluateObserverRadio(obs, "910.425,62.5,7,5", live, false)

	later := time.Now().UTC().Format(time.RFC3339)
	st.EvaluateObserverRadio(obs, "910.425,62.5,7,5", later, true)
	if got := radioOKAt(t, st, obs); got != live {
		t.Errorf("radio_ok_at = %q after a retained status, want the live one %q kept", got, live)
	}

	// With no anchor at all, a retained status may set one — otherwise there
	// is nothing to retract from.
	const fresh = "obs-B"
	if err := st.UpsertObserverStatus(fresh, fresh, "YVR", "", "{}", "910.425,62.5,7,5", later); err != nil {
		t.Fatal(err)
	}
	st.EvaluateObserverRadio(fresh, "910.425,62.5,7,5", later, true)
	if got := radioOKAt(t, st, fresh); got != later {
		t.Errorf("radio_ok_at = %q with no prior anchor, want %q", got, later)
	}
}
