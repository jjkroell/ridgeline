package api

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRespCacheReusesWithinTTL(t *testing.T) {
	c := newRespCache(10)
	var calls int32
	compute := func() (any, error) { atomic.AddInt32(&calls, 1); return map[string]int{"n": 1}, nil }
	a, _ := c.get("k", time.Minute, compute)
	b, _ := c.get("k", time.Minute, compute)
	if calls != 1 {
		t.Errorf("computed %d times within the TTL, want 1", calls)
	}
	if string(a) != string(b) || string(a) != "{\"n\":1}\n" {
		t.Errorf("bodies %q / %q, want the writeJSON encoding", a, b)
	}
}

func TestRespCacheRecomputesAfterTTL(t *testing.T) {
	c := newRespCache(10)
	var calls int32
	compute := func() (any, error) { return atomic.AddInt32(&calls, 1), nil }
	c.get("k", 20*time.Millisecond, compute)
	time.Sleep(30 * time.Millisecond)
	body, _ := c.get("k", 20*time.Millisecond, compute)
	if calls != 2 || string(body) != "2\n" {
		t.Errorf("calls=%d body=%q, want a fresh computation after the TTL", calls, body)
	}
}

// The point of the cache: a burst of identical requests runs ONE scan.
func TestRespCacheSharesConcurrentComputation(t *testing.T) {
	c := newRespCache(10)
	var calls int32
	release := make(chan struct{})
	compute := func() (any, error) {
		atomic.AddInt32(&calls, 1)
		<-release
		return "x", nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.get("k", time.Minute, compute) }()
	}
	time.Sleep(50 * time.Millisecond) // let them all arrive while the first is running
	close(release)
	wg.Wait()
	if calls != 1 {
		t.Errorf("20 concurrent requests ran %d computations, want 1", calls)
	}
}

func TestRespCacheDoesNotCacheErrors(t *testing.T) {
	c := newRespCache(10)
	var calls int32
	fail := true
	compute := func() (any, error) {
		atomic.AddInt32(&calls, 1)
		if fail {
			return nil, errors.New("db busy")
		}
		return "ok", nil
	}
	if _, err := c.get("k", time.Minute, compute); err == nil {
		t.Fatal("want the error passed through")
	}
	fail = false
	body, err := c.get("k", time.Minute, compute)
	if err != nil || string(body) != "\"ok\"\n" || calls != 2 {
		t.Errorf("after a failure: body=%q err=%v calls=%d, want a retry that succeeds", body, err, calls)
	}
}

func TestRespCacheStaysBounded(t *testing.T) {
	c := newRespCache(50)
	for i := 0; i < 500; i++ {
		c.get(fmt.Sprintf("k%d", i), time.Minute, func() (any, error) { return i, nil })
	}
	if n := len(c.entries); n > 51 {
		t.Errorf("cache holds %d entries, want it capped near 50", n)
	}
}
