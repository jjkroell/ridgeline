package ingest

import (
	"sync"
	"time"
)

// A node row is created by the first signature-valid advert from an unknown
// key. A signature proves only that the sender holds that key, and keys are
// free: one publisher can mint a fresh key per advert and grow the nodes table
// (and every full node-list scan) without limit. newNodeRation bounds how many
// nodes any one observer may introduce. Adverts past the ration are still
// stored as observations; they just do not add a node row until the observer
// has budget again (or another observer reports the same key).
//
// The burst covers an empty database meeting a busy mesh, where every node a
// first observer hears is new; after that one genuinely new node a minute per
// observer is far above anything a real mesh produces.
const (
	newNodeBurst = 200
	newNodeRate  = 1.0 / 60 // per second
)

type newNodeRation struct {
	mu      sync.Mutex
	buckets map[string]*newNodeBucket
}

type newNodeBucket struct {
	tokens float64
	last   time.Time
}

// allow consumes one new-node token for observerID at now.
func (r *newNodeRation) allow(observerID string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.buckets == nil {
		r.buckets = map[string]*newNodeBucket{}
	}
	b := r.buckets[observerID]
	if b == nil {
		// Bound the map: drop buckets idle long enough to have refilled.
		if len(r.buckets) > 10000 {
			for k, ob := range r.buckets {
				if now.Sub(ob.last).Seconds()*newNodeRate >= newNodeBurst {
					delete(r.buckets, k)
				}
			}
		}
		b = &newNodeBucket{tokens: newNodeBurst, last: now}
		r.buckets[observerID] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * newNodeRate
	if b.tokens > newNodeBurst {
		b.tokens = newNodeBurst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
