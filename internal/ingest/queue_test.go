package ingest

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/config"
	"github.com/jjkroell/ridgeline/internal/store"
)

func queueIngestor(t *testing.T) *Ingestor {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(config.MQTT{}, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (in *Ingestor) startWorker() {
	in.working.Store(true)
	go in.work()
}

func queuePacket(i int) fakeMsg {
	return fakeMsg{
		topic:   "meshcore/TST/" + strings.Repeat("AB", 32) + "/packets",
		payload: fmt.Sprintf(`{"raw":"1100%08X","origin":"t"}`, i),
	}
}

// The regression: with the store busy, the message callback must return at once
// (it runs in the goroutine that reads PINGRESP), and nothing may be lost,
// reordered, or stamped with the time it was finally handled.
func TestReceiveDoesNotWaitForTheStore(t *testing.T) {
	in := queueIngestor(t)
	release := make(chan struct{})
	var got []store.Observation
	in.commitFn = func(o store.Observation) bool { <-release; got = append(got, o); return true }
	in.startWorker()

	const n = 500
	began := time.Now()
	for i := range n {
		in.receive(nil, queuePacket(i))
	}
	received := time.Now()
	if took := received.Sub(began); took > time.Second {
		t.Fatalf("receiving %d messages took %v with the store blocked", n, took)
	}
	time.Sleep(200 * time.Millisecond) // the stall
	close(release)
	in.Stop() // drains

	if len(got) != n {
		t.Fatalf("handled %d of %d", len(got), n)
	}
	for i, o := range got {
		if want := fmt.Sprintf("1100%08X", i); o.RawHex != want {
			t.Fatalf("message %d out of order: got %s want %s", i, o.RawHex, want)
		}
		if o.ReceivedAt.Before(began) || o.ReceivedAt.After(received) {
			t.Fatalf("message %d stamped %v, outside its arrival %v..%v", i, o.ReceivedAt, began, received)
		}
	}
	if b := in.queuedBytes.Load(); b != 0 {
		t.Errorf("%d bytes still accounted after the queue emptied", b)
	}
}

// A full queue drops the newest without blocking; memory stays bounded.
func TestReceiveIsBounded(t *testing.T) {
	in := queueIngestor(t)
	for i := range queueLen + 25 { // no worker running
		in.receive(nil, queuePacket(i))
	}
	if len(in.queue) != queueLen || in.queueFull.Load() != 25 {
		t.Fatalf("queued %d, refused %d", len(in.queue), in.queueFull.Load())
	}

	in = queueIngestor(t)
	in.receive(nil, fakeMsg{payload: strings.Repeat("x", maxQueuedMessage+1)})
	if len(in.queue) != 0 {
		t.Error("a message no handler accepts was queued")
	}
	for range maxQueueBytes/maxQueuedMessage + 10 {
		in.receive(nil, fakeMsg{payload: strings.Repeat("x", maxQueuedMessage)})
	}
	if got := in.queuedBytes.Load(); got > maxQueueBytes {
		t.Errorf("queue holds %d bytes, limit %d", got, maxQueueBytes)
	}
	if in.queueFull.Load() != 10 {
		t.Errorf("refused %d, want 10", in.queueFull.Load())
	}
}

// Stop must not hang on a handler that never finishes.
func TestStopIsBounded(t *testing.T) {
	in := queueIngestor(t)
	stuck := make(chan struct{})
	defer close(stuck)
	in.commitFn = func(store.Observation) bool { <-stuck; return true }
	in.startWorker()
	in.receive(nil, queuePacket(1))
	began := time.Now()
	in.Stop()
	if took := time.Since(began); took > drainLimit+3*time.Second {
		t.Fatalf("Stop took %v", took)
	}
}

func TestLagReportsOnceEachWay(t *testing.T) {
	var l lagReport
	var seen []lagChange
	for _, w := range []time.Duration{0, 5 * time.Second, 16 * time.Second, 40 * time.Second, 10 * time.Second, 500 * time.Millisecond, 0} {
		if c := l.observe(w); c != lagNone {
			seen = append(seen, c)
		}
	}
	if len(seen) != 2 || seen[0] != lagStarted || seen[1] != lagEnded || l.lastPeak != 40*time.Second {
		t.Fatalf("changes %v, peak %v", seen, l.lastPeak)
	}
}
