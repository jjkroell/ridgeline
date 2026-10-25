package api

import (
	"sync"
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

// nodeListTTL is how long /api/nodes and node detail reuse one node-list scan.
// Each ListNodes reads the whole nodes table on the single DB connection and
// post-processes it (outliers, bridge segments, radios), so per-request scans
// let a burst of anonymous requests queue that work back to back. A few
// seconds of staleness on a list that changes by the advert is invisible.
const nodeListTTL = 5 * time.Second

// nodeList is a one-entry cache of store.ListNodes. Callers share the slice
// and must treat it as read-only.
type nodeList struct {
	mu    sync.Mutex // held across the scan, so concurrent misses share one
	at    time.Time
	nodes []store.Node
}

func (s *Server) cachedNodes() ([]store.Node, error) {
	c := &s.nodeList
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nodes != nil && time.Since(c.at) < nodeListTTL {
		return c.nodes, nil
	}
	nodes, err := s.store.ListNodes()
	if err != nil {
		return nil, err
	}
	if nodes == nil {
		nodes = []store.Node{}
	}
	c.nodes, c.at = nodes, time.Now()
	return nodes, nil
}

func (c *nodeList) invalidate() {
	c.mu.Lock()
	c.nodes = nil
	c.mu.Unlock()
}
