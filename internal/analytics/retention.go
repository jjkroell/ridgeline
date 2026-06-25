package analytics

import "github.com/jjkroell/ridgeline/internal/store"

// StaleNodeKeys returns the public keys of nodes that have gone silent past the
// retention cutoff and should be pruned. A node is stale when its last advert is
// older than cutoffISO (an RFC3339Nano UTC timestamp) AND it isn't in keep — the
// set of nodes seen relaying within the recent liveness window.
//
// last_seen on a node row tracks only its own adverts, and a healthy MeshCore
// node re-adverts every few hours, so an advert silence measured in weeks is a
// reliable "gone" signal. keep guards the rare node whose advert is stale but
// which is still forwarding traffic (so still part of the mesh) from being cut.
func StaleNodeKeys(nodes []store.Node, keep map[string]LiveSignal, cutoffISO string) []string {
	var stale []string
	for _, n := range nodes {
		if n.LastSeen == "" || n.LastSeen >= cutoffISO {
			continue // never seen (shouldn't happen) or adverted recently enough
		}
		if _, alive := keep[n.PublicKey]; alive {
			continue // stale advert, but still relaying — keep it
		}
		stale = append(stale, n.PublicKey)
	}
	return stale
}
