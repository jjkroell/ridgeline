package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// respCache holds the encoded JSON of expensive, public, per-node responses
// (heatmap, observers, history) for a short time, and makes concurrent requests
// for the same key share ONE computation.
//
// Why: each of those endpoints scans days of observations on the single DB
// connection. Measured on prod (2026-10-07), one node page cost ~11s of that
// connection (heatmap 5.0s, observers 3.2s, history 2.2s), so a few visitors
// browsing node pages stalled the whole site for 6–12s. The data is a
// rolling window, so a minute or a few of staleness is invisible.
//
// Only for responses that depend on nothing but the URL: never anything
// user-specific (claims, notes, private locations).
type respCache struct {
	mu      sync.Mutex
	entries map[string]*respEntry
	max     int
}

type respEntry struct {
	done chan struct{} // closed when body/err are set
	body []byte
	err  error
	at   time.Time
}

func newRespCache(max int) *respCache {
	return &respCache{entries: map[string]*respEntry{}, max: max}
}

// get returns the cached encoding for key if it is younger than ttl; otherwise
// it runs compute once (other callers for the same key wait for that run) and
// caches the result. Errors are returned to every waiter but never cached.
func (c *respCache) get(key string, ttl time.Duration, compute func() (any, error)) ([]byte, error) {
	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		select {
		case <-e.done:
			if e.err == nil && time.Since(e.at) < ttl {
				c.mu.Unlock()
				return e.body, nil
			}
			// stale or failed: fall through and recompute
		default:
			// in flight: wait for it rather than starting a second scan
			c.mu.Unlock()
			<-e.done
			return e.body, e.err
		}
	}
	e := &respEntry{done: make(chan struct{})}
	c.entries[key] = e
	c.evictLocked(ttl)
	c.mu.Unlock()

	v, err := compute()
	var body []byte
	if err == nil {
		var buf bytes.Buffer
		// Same encoding as writeJSON (json.Encoder, trailing newline).
		if err = json.NewEncoder(&buf).Encode(v); err == nil {
			body = buf.Bytes()
		}
	}

	c.mu.Lock()
	e.body, e.err, e.at = body, err, time.Now()
	close(e.done)
	if err != nil && c.entries[key] == e {
		delete(c.entries, key) // let the next request retry
	}
	c.mu.Unlock()
	return body, err
}

// evictLocked keeps the map bounded: drop finished entries older than ttl, and
// if that is not enough, drop finished entries until under the cap. In-flight
// entries are never dropped (their waiters hold the entry itself anyway).
func (c *respCache) evictLocked(ttl time.Duration) {
	if len(c.entries) <= c.max {
		return
	}
	for k, e := range c.entries {
		select {
		case <-e.done:
			if time.Since(e.at) >= ttl {
				delete(c.entries, k)
			}
		default:
		}
	}
	for k, e := range c.entries {
		if len(c.entries) <= c.max {
			return
		}
		select {
		case <-e.done:
			delete(c.entries, k)
		default:
		}
	}
}

// writeCachedJSON serves a body produced by respCache.get.
func writeCachedJSON(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}
