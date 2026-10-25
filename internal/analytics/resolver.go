package analytics

import (
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

// newPrefixResolver returns a function mapping a path-hop hex prefix to the
// public key of the unique node it identifies, or "" when no node — or more
// than one — matches at that prefix length. Hops use the ORIGINATING node's
// hash size, so prefixes vary in length (2/4/6 hex chars); we index every node
// at each length and only resolve unambiguous matches.
//
// ⚠ ACCURACY CAVEAT — 1-byte hops are weak evidence, and this resolver accepts
// them. Uniqueness guards against AMBIGUITY (two known nodes sharing a prefix)
// but not against SATURATION: there are only 256 one-byte values and a busy mesh
// exercises ~97% of them within a week, so a 1-byte hop matching exactly one
// KNOWN node may still have been written by a node we have never seen. Measured
// on the live mesh, ~5% of all hop attributions come through that path.
//
// That is tolerable for the aggregates built on it (relay counts, activity
// heatmaps, per-node history) where it is a small statistical smear. It is NOT
// tolerable anywhere a single false attribution has an outsized, binary
// consequence — node retention (see analytics.FilterRelayedWithin) and
// injection detection both require ≥2-byte evidence via newPrefixResolverMin.
func newPrefixResolver(nodes []store.Node) func(hop string) string {
	return newPrefixResolverMin(nodes, 1)
}

// newPrefixResolverMin is newPrefixResolver restricted to hops of at least
// minBytes wide; narrower hops resolve to "". Use this wherever a wrong
// attribution costs more than a missing one.
func newPrefixResolverMin(nodes []store.Node, minBytes int) func(hop string) string {
	if minBytes < 1 {
		minBytes = 1
	}
	index := map[int]map[string][]int{2: {}, 4: {}, 6: {}}
	for i, n := range nodes {
		pk := strings.ToUpper(n.PublicKey)
		for _, l := range []int{2, 4, 6} {
			if len(pk) >= l {
				index[l][pk[:l]] = append(index[l][pk[:l]], i)
			}
		}
	}
	return func(hop string) string {
		h := strings.ToUpper(hop)
		if len(h)/2 < minBytes {
			return "" // too narrow to attribute — see the caveat above
		}
		m, ok := index[len(h)]
		if !ok {
			return ""
		}
		if i, ok := soleOwner(nodes, m[h]); ok {
			return nodes[i].PublicKey
		}
		return ""
	}
}

// soleOwner picks the one node a hop belongs to among the candidate indices
// whose key carries it. A single candidate owns it outright. Among several,
// rows that are not established are set aside: any signature-valid advert
// creates a row, so one advert from a key ground to a genuine node's prefix
// would otherwise make every hop of that node ambiguous — erasing its relay
// credit, getting it purged by retention, and hiding a bridge from injection
// detection. Two established rows sharing the prefix stay ambiguous.
func soleOwner(nodes []store.Node, cands []int) (int, bool) {
	if len(cands) == 1 {
		return cands[0], true
	}
	owner, n := -1, 0
	for _, i := range cands {
		if established(nodes[i]) {
			owner, n = i, n+1
		}
	}
	return owner, n == 1
}

// established reports whether a node row has a history a single injected
// advert cannot fake: several separate transmissions over at least a day.
func established(n store.Node) bool {
	if heard(n) < 3 {
		return false
	}
	first, err1 := time.Parse(time.RFC3339Nano, n.FirstSeen)
	last, err2 := time.Parse(time.RFC3339Nano, n.LastSeen)
	return err1 == nil && err2 == nil && last.Sub(first) >= 24*time.Hour
}

// displayName returns a node's name, falling back to its key when unnamed.
func displayName(n store.Node, key string) string {
	if n.Name != "" {
		return n.Name
	}
	return key
}
