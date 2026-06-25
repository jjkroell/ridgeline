package analytics

import (
	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// ConsensusHashSizes scans the advert window and returns each node's hash size
// by majority vote of its adverts' path-length bytes, keyed by pubkey. Only
// nodes with a clear winner are included.
//
// A node's true hash size is fixed, but a corrupt advert (a flipped path-length
// byte) reports a wrong size, and ingest takes only the latest advert — so one
// corrupt packet can flip the stored value. Voting over every advert in the
// window (direct and relayed alike independently report the originator's size)
// recovers the real size and ignores the rare corrupt outlier.
func ConsensusHashSizes(st *store.Store, cutoffISO string) (map[string]int, error) {
	raws, err := st.RawWindow(cutoffISO, 0)
	if err != nil {
		return nil, err
	}
	votes := map[string]map[int]int{} // pubkey -> size -> count
	for _, ro := range raws {
		pkt, err := meshcore.DecodeHex(ro.RawHex)
		if err != nil || pkt == nil || pkt.Advert == nil || pkt.Advert.PublicKey == "" {
			continue
		}
		hs := pkt.PathHashSize
		if hs < 1 || hs > 3 {
			continue
		}
		m := votes[pkt.Advert.PublicKey]
		if m == nil {
			m = map[int]int{}
			votes[pkt.Advert.PublicKey] = m
		}
		m[hs]++
	}

	out := make(map[string]int, len(votes))
	for pk, m := range votes {
		size, ok := confidentMode(m)
		if ok {
			out[pk] = size
		}
	}
	return out, nil
}

// confidentMode returns the size with the most votes and whether it's a clear
// winner: at least two votes and a strict majority of the total. This keeps a
// lone corrupt advert from being treated as the verdict while a node with too
// few or evenly-split adverts is left untouched.
func confidentMode(votes map[int]int) (int, bool) {
	best, bestN, total := 0, 0, 0
	for size, n := range votes {
		total += n
		if n > bestN || (n == bestN && size < best) {
			best, bestN = size, n
		}
	}
	if bestN < 2 || bestN*2 <= total {
		return 0, false
	}
	return best, true
}
