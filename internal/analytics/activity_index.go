package analytics

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// ActivityIndex keeps, for every node, how many observations it was part of in
// each UTC hour over a rolling window — the data behind the node page's
// weekday×hour heatmap — so serving a heatmap is a sum over ≤168 cells in
// memory instead of decoding days of packets per view.
//
// Why: NodeHeatmap scanned RawWindow on every view (~5 s on prod, on the single
// DB connection), and because RawWindow returns the NEWEST 200k rows, at
// ~100k observations/day the "last 7d" heatmap really covered ~2 days.
//
// Attribution is NodeHeatmap's: a node's own adverts plus packets with a relay
// hop that resolves uniquely to it. Hops are resolved against the node list at
// the time the row is indexed rather than at view time — the same answer except
// while a colliding prefix is being added or removed.
type ActivityIndex struct {
	window time.Duration

	mu      sync.RWMutex
	buckets map[string]map[int64]int // UPPER pubkey -> unix hour -> count
	lastID  int64
	ready   bool
}

// NewActivityIndex returns an empty index covering window.
func NewActivityIndex(window time.Duration) *ActivityIndex {
	return &ActivityIndex{window: window, buckets: map[string]map[int64]int{}}
}

// Window is how far back the index answers.
func (a *ActivityIndex) Window() time.Duration { return a.window }

// activityBatch bounds one incremental read; activityPause yields the
// connection between the startup backfill's one-hour reads (~700k rows over
// 7 days on prod).
const (
	activityBatch = 5000
	activityPause = 20 * time.Millisecond
)

// Update indexes every observation stored since the last call, then drops
// hours that have left the window. The first call backfills the window by
// TIME (one-hour index ranges, capped at the id that was newest when it
// started) because ids do not strictly follow reception time; every call after
// that pages forward by id from there, so a row is counted exactly once
// whatever timestamp it carries. Either way the DB connection is released
// between batches.
func (a *ActivityIndex) Update(ctx context.Context, st *store.Store, nodes []store.Node, now time.Time) error {
	a.mu.RLock()
	last, ready := a.lastID, a.ready
	a.mu.RUnlock()

	resolve := newPrefixResolver(nodes)
	cutoff := now.Add(-a.window).Unix() / 3600

	if !ready {
		maxID, err := st.MaxObservationID()
		if err != nil {
			return err
		}
		end := now.Add(time.Minute)
		for from := now.Add(-a.window).Truncate(time.Hour); from.Before(end); from = from.Add(time.Hour) {
			if err := ctx.Err(); err != nil {
				return err
			}
			rows, err := st.RawBetween(from.UTC().Format(time.RFC3339Nano), from.Add(time.Hour).UTC().Format(time.RFC3339Nano), maxID)
			if err != nil {
				return err
			}
			a.addRows(rows, resolve, cutoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(activityPause):
			}
		}
		last = maxID
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := st.RawAfterID(last, activityBatch)
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			last = rows[len(rows)-1].ID
		}
		a.addRows(rows, resolve, cutoff)
		if len(rows) < activityBatch {
			break
		}
	}

	a.mu.Lock()
	a.lastID = last
	for k, hours := range a.buckets {
		for h := range hours {
			if h < cutoff {
				delete(hours, h)
			}
		}
		if len(hours) == 0 {
			delete(a.buckets, k)
		}
	}
	a.ready = true
	a.mu.Unlock()
	return nil
}

// addRows decodes a batch (outside the lock) and adds it to the buckets.
func (a *ActivityIndex) addRows(rows []store.RawRow, resolve func(string) string, cutoff int64) {
	add := map[string]map[int64]int{}
	for _, r := range rows {
		t := parseTime(r.ReceivedAt)
		if t.IsZero() {
			continue
		}
		hour := t.UTC().Unix() / 3600
		if hour < cutoff {
			continue
		}
		pkt, err := meshcore.DecodeHex(r.RawHex)
		if err != nil || pkt == nil {
			continue
		}
		for _, k := range attributedNodes(pkt, resolve) {
			if add[k] == nil {
				add[k] = map[int64]int{}
			}
			add[k][hour]++
		}
	}
	if len(add) == 0 {
		return
	}
	a.mu.Lock()
	for k, hours := range add {
		if a.buckets[k] == nil {
			a.buckets[k] = map[int64]int{}
		}
		for h, n := range hours {
			a.buckets[k][h] += n
		}
	}
	a.mu.Unlock()
}

// attributedNodes is who a packet counts towards: its advert's origin, and
// every relay hop that resolves to exactly one node (each node once).
func attributedNodes(pkt *meshcore.Packet, resolve func(string) string) []string {
	var out []string
	seen := map[string]bool{}
	if pkt.Advert != nil && pkt.Advert.PublicKey != "" {
		k := strings.ToUpper(pkt.Advert.PublicKey)
		seen[k] = true
		out = append(out, k)
	}
	for _, hop := range pkt.RelayPath() {
		k := strings.ToUpper(resolve(hop))
		if k != "" && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// Heatmap returns pubkey's weekday×hour grid over the last days days. ok is
// false until the first Update has finished, or when days exceeds the window —
// the caller falls back to scanning.
func (a *ActivityIndex) Heatmap(pubkey string, days int, now time.Time) (*NodeActivity, bool) {
	if time.Duration(days)*24*time.Hour > a.window {
		return nil, false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.ready {
		return nil, false
	}
	out := &NodeActivity{Days: days}
	from := now.Add(-time.Duration(days)*24*time.Hour).Unix() / 3600
	for h, n := range a.buckets[strings.ToUpper(pubkey)] {
		if h < from {
			continue
		}
		t := time.Unix(h*3600, 0).UTC()
		out.Grid[int(t.Weekday())][t.Hour()] += n
		out.Total += n
	}
	for d := 0; d < 7; d++ {
		for hr := 0; hr < 24; hr++ {
			if out.Grid[d][hr] > out.Max {
				out.Max = out.Grid[d][hr]
			}
		}
	}
	return out, true
}
