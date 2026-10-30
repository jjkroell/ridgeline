package store

import (
	"context"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

func TestMonitorWord(t *testing.T) {
	for msg, want := range map[string]monitorKind{
		"test":                monitorTest,
		"Test!":               monitorTest,
		"TEST 1":              monitorTest,
		"  test from Nanaimo": monitorTest,
		"test.":               monitorTest,
		"unsub":               monitorUnsub,
		"Unsubscribe please":  monitorUnsub,
		"UNSUB!":              monitorUnsub,
		"testing":             notMonitor,
		"contest":             notMonitor,
		"a test":              notMonitor,
		"unsubscribed":        notMonitor,
		"":                    notMonitor,
	} {
		if got := monitorWord(msg); got != want {
			t.Errorf("monitorWord(%q) = %v, want %v", msg, got, want)
		}
	}
}

// unsub takes a sender off; only a NEWER test puts them back, whatever order
// the copies arrive in.
func TestSOSUnsub(t *testing.T) {
	RegisterSOSChannel()
	st := testStore(t)
	key := meshcore.DeriveHashtagKey(SOSChannel)
	now := time.Now().UTC()

	oldTest := groupTextRaw(t, key, "Erin", "test")
	recordRaw(t, st, oldTest, now)
	recordRaw(t, st, groupTextRaw(t, key, "Erin", "unsub"), now.Add(time.Minute))
	if n := len(st.SOSMonitors()); n != 0 {
		t.Fatalf("after unsub: %d monitors, want 0", n)
	}
	// A late observer copy of the OLD test, stored after the unsub.
	recordRaw(t, st, oldTest, now.Add(-time.Second))
	if n := len(st.SOSMonitors()); n != 0 {
		t.Fatalf("late old test re-listed: %d monitors", n)
	}
	// An unsub from someone never listed leaves nobody listed.
	recordRaw(t, st, groupTextRaw(t, key, "Finn", "unsubscribe"), now)
	recordRaw(t, st, groupTextRaw(t, key, "Erin", "Test again"), now.Add(2*time.Minute))
	mons := st.SOSMonitors()
	if len(mons) != 1 || mons[0].Name != "Erin" {
		t.Fatalf("after re-test = %+v, want Erin", mons)
	}
	// The table agrees with memory.
	if err := st.loadSOSMonitors(); err != nil {
		t.Fatal(err)
	}
	if mons := st.SOSMonitors(); len(mons) != 1 || mons[0].Name != "Erin" {
		t.Fatalf("after reload = %+v", mons)
	}
	// Finn tests later: first_test comes from the test, not the unsub's ''.
	recordRaw(t, st, groupTextRaw(t, key, "Finn", "test"), now.Add(3*time.Minute))
	if err := st.loadSOSMonitors(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range st.SOSMonitors() {
		if m.Name == "Finn" {
			found = true
			if m.FirstTest == "" {
				t.Fatal("Finn's first_test is empty")
			}
		}
	}
	if !found {
		t.Fatal("Finn not listed after testing")
	}
}

// A live #sos "test" lists its sender once per transmission (not per observer
// copy); other messages and other channels do not.
func TestRecordSOSMonitors(t *testing.T) {
	RegisterSOSChannel()
	st := testStore(t)
	key := meshcore.DeriveHashtagKey(SOSChannel)
	now := time.Now().UTC()

	test1 := groupTextRaw(t, key, "Alice", "test")
	recordRaw(t, st, test1, now)
	recordRaw(t, st, test1, now.Add(2*time.Second)) // another observer's copy
	recordRaw(t, st, groupTextRaw(t, key, "Alice", "Test from the ridge"), now.Add(time.Minute))
	recordRaw(t, st, groupTextRaw(t, key, "Bob", "anyone around?"), now)
	pub := []byte{0x8b, 0x33, 0x87, 0xe9, 0xc5, 0xcd, 0xea, 0x6a, 0xc9, 0xe5, 0xed, 0xba, 0xa1, 0x15, 0xcd, 0x72}
	recordRaw(t, st, groupTextRaw(t, pub, "Carol", "test"), now) // Public, not #sos

	mons := st.SOSMonitors()
	if len(mons) != 1 || mons[0].Name != "Alice" || mons[0].Tests != 2 {
		t.Fatalf("monitors = %+v, want Alice with 2 tests", mons)
	}

	// The roster survives a reopen (loaded from the table).
	if err := st.loadSOSMonitors(); err != nil {
		t.Fatal(err)
	}
	if mons := st.SOSMonitors(); len(mons) != 1 || mons[0].Tests != 2 {
		t.Fatalf("after reload = %+v", mons)
	}
}

// The backfill finds tests already in history, runs once, and agrees with
// rows live ingest wrote for the same transmission.
func TestBackfillSOSMonitors(t *testing.T) {
	RegisterSOSChannel()
	st := testStore(t)
	key := meshcore.DeriveHashtagKey(SOSChannel)
	now := time.Now().UTC()

	recordRaw(t, st, groupTextRaw(t, key, "Dave", "test"), now.Add(-48*time.Hour))
	// Wipe what live ingest wrote, as on a database from before the roster.
	if _, err := st.db.Exec(`DELETE FROM channel_monitors`); err != nil {
		t.Fatal(err)
	}
	if err := st.loadSOSMonitors(); err != nil {
		t.Fatal(err)
	}
	if len(st.SOSMonitors()) != 0 {
		t.Fatal("roster not cleared")
	}

	scanned, found, err := st.BackfillSOSMonitors(context.Background(), 7, now)
	if err != nil || scanned != 1 || found != 1 {
		t.Fatalf("backfill = %d scanned, %d found, %v", scanned, found, err)
	}
	if mons := st.SOSMonitors(); len(mons) != 1 || mons[0].Name != "Dave" || mons[0].Tests != 1 {
		t.Fatalf("monitors = %+v", mons)
	}
	// Second run is a no-op.
	if scanned, _, err := st.BackfillSOSMonitors(context.Background(), 7, now); err != nil || scanned != 0 {
		t.Fatalf("second backfill scanned %d, %v", scanned, err)
	}
}

// Same cases as the web client's ALERT_RE; the two must agree.
func TestSOSAlertWords(t *testing.T) {
	for msg, want := range map[string]bool{
		"SOS at Mt Seymour":       true,
		"emergency!":              true,
		"Need help near Squamish": true,
		"he needs help":           true,
		"help me":                 true,
		"call 911":                true,
		"Mayday":                  true,
		"sos":                     true,
		"announce #sos on Public": false,
		"thanks for the help":     false,
		"sosa":                    false,
		"test":                    false,
	} {
		if got := sosAlertRe.MatchString(msg); got != want {
			t.Errorf("alert(%q) = %v, want %v", msg, got, want)
		}
	}
}

// The newest #sos alert is noted live and found again after a restart, and
// only within the window.
func TestSOSLastAlert(t *testing.T) {
	RegisterSOSChannel()
	st := testStore(t)
	key := meshcore.DeriveHashtagKey(SOSChannel)
	now := time.Now().UTC()

	recordRaw(t, st, groupTextRaw(t, key, "Hiker", "SOS rolled ankle"), now.Add(-3*time.Hour))
	recordRaw(t, st, groupTextRaw(t, key, "Hiker", "on my way"), now.Add(-time.Hour))
	want := now.Add(-3 * time.Hour).Format(time.RFC3339Nano)
	if got := st.SOSLastAlert(); got != want {
		t.Fatalf("live lastAlert = %q, want %q", got, want)
	}

	st.sosLastAlert = "" // as after a restart
	if err := st.LoadSOSAlert(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if got := st.SOSLastAlert(); got != want {
		t.Fatalf("loaded lastAlert = %q, want %q", got, want)
	}

	st.sosLastAlert = ""
	if err := st.LoadSOSAlert(context.Background(), now.Add(13*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := st.SOSLastAlert(); got != "" {
		t.Fatalf("alert outside the window loaded: %q", got)
	}
}

// Purging a channel removes its observations (every observer's copy) and
// nothing else; the monitor roster survives.
func TestPurgeChannelKeepsRoster(t *testing.T) {
	RegisterSOSChannel()
	st := testStore(t)
	key := meshcore.DeriveHashtagKey(SOSChannel)
	pub := []byte{0x8b, 0x33, 0x87, 0xe9, 0xc5, 0xcd, 0xea, 0x6a, 0xc9, 0xe5, 0xed, 0xba, 0xa1, 0x15, 0xcd, 0x72}
	now := time.Now().UTC()

	test := groupTextRaw(t, key, "Gail", "test")
	recordRaw(t, st, test, now.Add(-time.Hour))
	recordRaw(t, st, test, now.Add(-time.Hour+time.Second)) // second observer
	recordRaw(t, st, groupTextRaw(t, key, "Gail", "anyone?"), now.Add(-30*time.Minute))
	recordRaw(t, st, groupTextRaw(t, pub, "Hal", "hello"), now.Add(-30*time.Minute))

	obs, err := st.ChannelObservations(context.Background(), SOSChannel, 1, now)
	if err != nil || len(obs) != 3 {
		t.Fatalf("found %d #sos observations (%v), want 3", len(obs), err)
	}
	ids := make([]int64, len(obs))
	for i, o := range obs {
		ids[i] = o.ID
	}
	if n, err := st.DeleteObservations(ids); err != nil || n != 3 {
		t.Fatalf("deleted %d (%v), want 3", n, err)
	}
	if left, _ := st.ChannelObservations(context.Background(), SOSChannel, 1, now); len(left) != 0 {
		t.Fatalf("%d #sos observations left", len(left))
	}
	var total int
	st.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&total)
	if total != 1 {
		t.Fatalf("%d observations left, want the 1 Public message", total)
	}
	if err := st.loadSOSMonitors(); err != nil {
		t.Fatal(err)
	}
	if mons := st.SOSMonitors(); len(mons) != 1 || mons[0].Name != "Gail" {
		t.Fatalf("roster after purge = %+v, want Gail", mons)
	}
}
