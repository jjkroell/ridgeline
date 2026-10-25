package api

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Public scan endpoints key their shared cache on query parameters; snapping
// keeps a caller from minting a fresh uncached scan by nudging an integer.
func TestSnapUpCollapsesNeighbouringValues(t *testing.T) {
	presets := []int{3600, 21600, 86400}
	for v, want := range map[int]int{1: 3600, 3600: 3600, 3601: 21600, 86398: 86400, 86400: 86400, 999999: 86400} {
		if got := snapUp(v, presets...); got != want {
			t.Errorf("snapUp(%d) = %d, want %d", v, got, want)
		}
	}
}

// An unknown pubkey or observer id must be refused before any window scan:
// the scans cost the same whether or not the key matches anything.
func TestScanEndpointsRejectUnknownKeys(t *testing.T) {
	_, base, cleanup := newAuthEnv(t)
	defer cleanup()
	bogus := strings.Repeat("AB", 32)
	for _, path := range []string{
		"/api/nodes/" + bogus + "/heatmap?days=30",
		"/api/nodes/" + bogus + "/history?since=604797&limit=999",
		"/api/nodes/" + bogus + "/observers?since=604790",
		"/api/observers/NOPE/analytics?since=604790",
	} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, resp.StatusCode)
		}
	}
}

// Distinct keys compute concurrently only up to maxConcurrentScans.
func TestRespCacheBoundsConcurrentComputes(t *testing.T) {
	c := newRespCache(100)
	var running, peak int32
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func(i int) {
			c.get(string(rune('a'+i)), time.Minute, func() (any, error) {
				n := atomic.AddInt32(&running, 1)
				for {
					p := atomic.LoadInt32(&peak)
					if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				atomic.AddInt32(&running, -1)
				return i, nil
			})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	if peak > maxConcurrentScans {
		t.Errorf("peak concurrent computes = %d, want <= %d", peak, maxConcurrentScans)
	}
}
