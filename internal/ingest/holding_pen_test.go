package ingest

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

func testIngestor() *Ingestor {
	return &Ingestor{
		log:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		done: make(chan struct{}),
	}
}

func obsAt(id string, t time.Time) store.Observation {
	return store.Observation{ObserverID: id, ReceivedAt: t}
}

func TestHoldThenRelease(t *testing.T) {
	in := testIngestor()
	base := time.Now().Add(-10 * time.Minute)
	for i := 0; i < 3; i++ {
		if !in.holdObservation("obs-1", obsAt("obs-1", base.Add(time.Duration(i)*time.Minute))) {
			t.Fatalf("hold %d refused", i)
		}
	}
	obsN, pkts := in.HeldCount()
	if obsN != 1 || pkts != 3 {
		t.Fatalf("held = %d observers / %d packets, want 1/3", obsN, pkts)
	}

	var committed []store.Observation
	in.commitFn = func(o store.Observation) bool { committed = append(committed, o); return true }
	in.releaseHeld("obs-1")

	if len(committed) != 3 {
		t.Fatalf("released %d, want 3", len(committed))
	}
	// Arrival order preserved, and each keeps the time it was HEARD — not the
	// moment we decided to believe the observer.
	for i := 1; i < len(committed); i++ {
		if !committed[i].ReceivedAt.After(committed[i-1].ReceivedAt) {
			t.Error("release reordered held observations")
		}
	}
	if !committed[0].ReceivedAt.Equal(base) {
		t.Errorf("ReceivedAt = %v, want the arrival time %v — re-stamping misfiles the packet",
			committed[0].ReceivedAt, base)
	}
	if o, p := in.HeldCount(); o != 0 || p != 0 {
		t.Errorf("pen not emptied: %d/%d", o, p)
	}
}

func TestHoldThenDiscard(t *testing.T) {
	in := testIngestor()
	in.holdObservation("bad-1", obsAt("bad-1", time.Now()))
	in.holdObservation("bad-1", obsAt("bad-1", time.Now()))

	var committed int
	in.commitFn = func(store.Observation) bool { committed++; return true }
	in.discardHeld("bad-1", "wrong preset")

	if committed != 0 {
		t.Errorf("discard committed %d observations — held traffic from a refused observer must never be stored", committed)
	}
	if o, p := in.HeldCount(); o != 0 || p != 0 {
		t.Errorf("pen not emptied: %d/%d", o, p)
	}
}

// The cap is a DoS control, not tidiness: without it anyone who can publish can
// open an observer id, never send a status, and stream into the daemon's heap.
func TestHoldIsBounded(t *testing.T) {
	in := testIngestor()
	accepted := 0
	for i := 0; i < holdPerObserver+250; i++ {
		if in.holdObservation("flood", obsAt("flood", time.Now())) {
			accepted++
		}
	}
	if accepted != holdPerObserver {
		t.Errorf("accepted %d, want exactly the cap %d", accepted, holdPerObserver)
	}
	if _, p := in.HeldCount(); p != holdPerObserver {
		t.Errorf("held %d packets, cap is %d", p, holdPerObserver)
	}
}

func TestReleaseOfUnknownObserverIsNoop(t *testing.T) {
	in := testIngestor()
	in.commitFn = func(store.Observation) bool { t.Fatal("nothing should be committed"); return false }
	in.releaseHeld("never-seen")
	in.discardHeld("never-seen", "n/a")
}

// Pens are per-observer: one observer's verdict must not release or destroy
// another's held traffic.
func TestPensAreIsolated(t *testing.T) {
	in := testIngestor()
	in.holdObservation("a", obsAt("a", time.Now()))
	in.holdObservation("b", obsAt("b", time.Now()))
	var got []string
	in.commitFn = func(o store.Observation) bool { got = append(got, o.ObserverID); return true }
	in.releaseHeld("a")
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("released %v, want only a", got)
	}
	if o, p := in.HeldCount(); o != 1 || p != 1 {
		t.Errorf("b's pen disturbed: %d/%d", o, p)
	}
}
