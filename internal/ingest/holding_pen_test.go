package ingest

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
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

// The sweeper's lifetime is the Ingestor's, not the broker's. It once started
// only in the connect-timeout branch, so on every normal startup it never ran:
// pens never expired, nothing was ever logged, and the grace-period quarantine
// could not fire. The only symptom was silence, which is why this asserts the
// goroutine is actually running rather than that Start() returned.
func TestSweeperRunsOnNormalStart(t *testing.T) {
	in := testIngestor()
	started := make(chan struct{})
	in.sweepHook = func() { close(started) }
	go in.sweepPens()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("pen sweeper did not run")
	}
	in.Stop()
}

// And it must stop when the Ingestor does, or a restarted daemon leaks one
// sweeper per Ingestor for the life of the process.
func TestSweeperStopsWithIngestor(t *testing.T) {
	in := testIngestor()
	done := make(chan struct{})
	go func() { in.sweepPens(); close(done) }()
	in.Stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pen sweeper did not exit on Stop")
	}
}

// Observer keys are self-minted, so the per-observer cap alone bounds nothing:
// many pens together must stay under holdTotal, and the count must come back
// down as pens are released or discarded.
func TestHoldTotalBoundedAcrossObservers(t *testing.T) {
	in := testIngestor()
	accepted := 0
	for o := 0; o < holdTotal/holdPerObserver+5; o++ {
		id := "flood-" + string(rune('A'+o%26)) + string(rune('a'+o/26))
		for i := 0; i < holdPerObserver; i++ {
			if in.holdObservation(id, obsAt(id, time.Now())) {
				accepted++
			}
		}
	}
	if accepted != holdTotal {
		t.Fatalf("accepted %d across all pens, want exactly holdTotal=%d", accepted, holdTotal)
	}
	in.discardHeld("flood-Aa", "test")
	if !in.holdObservation("late", obsAt("late", time.Now())) {
		t.Error("room freed by a discard should be usable again")
	}
}

// No MeshCore frame exceeds the radio MTU; ingest must refuse longer raw hex
// before it reaches the store (or a pen).
func TestIngestRefusesFramesOverMTU(t *testing.T) {
	in, _ := nbIngestor(t)
	var committed []int
	in.commitFn = func(o store.Observation) bool { committed = append(committed, len(o.RawHex)); return true }
	topic := "meshcore/TST/" + strings.Repeat("AB", 32) + "/packets"
	for _, n := range []int{100, meshcore.MaxPacketSize, meshcore.MaxPacketSize + 1, 16384} {
		raw := "1100" + strings.Repeat("00", n-2)
		in.handle(nil, fakeMsg{topic: topic, payload: `{"raw":"` + raw + `","origin":"t"}`})
	}
	if len(committed) != 2 || committed[0] != 200 || committed[1] != 2*meshcore.MaxPacketSize {
		t.Fatalf("want only the 100- and 255-byte frames stored, got hex lengths %v", committed)
	}
}
