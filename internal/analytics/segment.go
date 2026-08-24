package analytics

// Which nodes live BEYOND a sanctioned bridge?
//
// Observers are sorted into the segment they can hear, by matching the radio
// config they report against the one the operator declared for the bridge's far
// side (see internal/radio — the same channel arrives spelled several ways, so
// this is a numeric comparison, never a string one). That produces two kinds of
// evidence, and the strong kind is new:
//
//   - A FAR-SIDE observer hears the far segment directly. A zero-hop sighting
//     there is a node transmitting on that frequency, received on that
//     frequency: membership is measured, not inferred, and no path analysis is
//     involved. This is reported as confidence "observed".
//
//   - With no receiver over there, membership can only be INFERRED from the
//     traffic that crosses the bridge, which is the original rule below.
//
// The inferred rule stands unchanged for every deployment with no far-side
// receiver, and it is still what classifies a far-side node that no far-side
// observer happens to hear. Its three properties:
//
//  1. DIRECTION. Relays append their hash as a packet travels, so the path is
//     ordered and index(near) < index(far) IS the near->far direction. Traffic
//     crossing the other way is a different fact and is not counted.
//
//  2. WIDTH. The path hash width is the ORIGINATING node's setting, and every
//     relay appends at that width — so a narrow originator produces narrow hops
//     for the bridge too. A 2-byte hop identifies a bridge end uniquely on a
//     mesh this size; a 1-byte hop usually does not. Rather than discard narrow
//     traffic (which would make a whole class of node invisible — companions
//     skew narrow), a 1-byte crossing is admitted as PROBABLE, and only when the
//     FAR end's single byte is unique among known nodes. The near end may be
//     ambiguous; the far end carries the claim.
//
//  3. NEVER HEARD DIRECTLY BY A NEAR-SIDE OBSERVER. A receiver on this side
//     cannot hear the far side directly, so one zero-hop sighting BY SUCH A
//     RECEIVER disqualifies a node outright. This is what separates "lives over
//     there" from "a packet happened to route through the bridge once", and it
//     is the strongest of the three.
//
//     ⚠ Which observer heard it is the whole point. Counting zero-hop sightings
//     from every observer alike was correct only while they all sat on one side;
//     the moment someone adds a receiver on the far segment, that observer hears
//     far-side nodes directly and the veto fires on exactly the nodes it was
//     built to find — silently, and reporting "heard directly on this side",
//     which would be false.
//
// The window matters as much as the rule: it must not reach back before the
// bridge existed. Beforehand its two ends were ordinary RF relays and transiting
// them said nothing about segments, so an over-long window blends two different
// topologies and misclassifies nodes that have since moved.

import (
	"strings"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/radio"
	"github.com/jjkroell/ridgeline/internal/store"
)

// segMinShare is the fraction of a node's path-carrying sightings that must
// have crossed the bridge. Not 100%: a re-flooded copy can arrive by an odd
// route. Combined with the zero-hop veto this is a strict test — every node
// found on the live mesh scored 100%.
const segMinShare = 0.90

// segMinSightings guards against a node with two sightings scoring 100% on
// nothing. A far-side node advertises regularly, so this costs real cases little.
const segMinSightings = 5

// SegmentReport is the outcome of one sweep, for logging and the admin console.
type SegmentReport struct {
	Members   []store.SegmentMember `json:"members"`
	Scanned   int                   `json:"scanned"`
	Crossings int                   `json:"crossings"`
	Reverse   int                   `json:"reverse"`
	// Rejected lists nodes that crossed but failed a test, with the reason —
	// the interesting half of the output when a node is missing from the map.
	Rejected map[string]string `json:"rejected,omitempty"`
	// FarObservers counts receivers found to be sitting on a far segment, and
	// Observed counts members established by one hearing the node directly.
	// Both are zero on a deployment with no far-side receiver, which is what
	// makes "did my new observer get recognised?" answerable from the log.
	FarObservers int `json:"farObservers"`
	Observed     int `json:"observed"`
}

type segStat struct {
	// zeroHop is a direct sighting by a NEAR-side observer: the disqualifying
	// one. zeroHopFar is a direct sighting by a receiver on the far segment: the
	// opposite, and proof of membership.
	zeroHop    int
	zeroHopFar int
	withPath   int
	confirmed  int
	probable   int
}

// farObserverSets sorts receivers onto the far side of each bridge, returning
// bridgeNear -> observer id -> true, and how many distinct receivers were placed
// on some far segment.
//
// A receiver belongs to a far segment when its reported radio and the operator's
// declared far-side config describe the same RF network — compared numerically,
// since one channel reaches us spelled several ways (910.4249877 and 910.425 are
// the same 910.425). An observer with no reported radio, and every observer when
// the bridge has no declared far side, stays near-side: that is the reading that
// preserves the old behaviour exactly, and the traffic-based rule still gets its
// chance.
func farObserverSets(links []store.BridgeLink, observers []store.Observer) (map[string]map[string]bool, int) {
	out := make(map[string]map[string]bool, len(links))
	seen := map[string]bool{}
	for _, l := range links {
		set := map[string]bool{}
		if l.PeerRadio != "" {
			for _, o := range observers {
				if o.Radio != "" && radio.SameSegmentString(o.Radio, l.PeerRadio) {
					set[o.ID] = true
					seen[o.ID] = true
				}
			}
		}
		out[l.Near] = set
	}
	return out, len(seen)
}

// verdict decides one node's membership for one bridge from its sighting counts,
// returning either a confidence or the reason it was rejected. Both empty means
// the node is not a candidate at all: it never crossed the bridge and was never
// heard on the far segment, so there is nothing to judge and nothing to report.
//
// The order of the tests is the substance. Direct reception decides before any
// path reasoning, because a receiver hearing a transmission on its own frequency
// is a measurement while a path is an inference — and when both sides claim to
// hear a node directly, that contradiction is surfaced rather than resolved.
func (s segStat) verdict() (confidence, reject string) {
	via := s.confirmed + s.probable
	if via == 0 && s.zeroHopFar == 0 {
		return "", ""
	}
	switch {
	case s.zeroHopFar > 0 && s.zeroHop > 0:
		return "", "heard directly from both sides"
	case s.zeroHopFar > 0:
		return "observed", ""
	case s.zeroHop > 0:
		return "", "heard directly on this side"
	case s.withPath < segMinSightings:
		return "", "too few sightings to judge"
	case float64(via)/float64(s.withPath) < segMinShare:
		return "", "only some traffic crosses the bridge"
	case s.confirmed > 0:
		return "confirmed", ""
	default:
		return "probable", ""
	}
}

// DetectSegments finds the nodes on the far side of each sanctioned bridge:
// heard directly by a receiver over there, or else reachable only across the
// bridge. sinceISO must not predate the bridge being put in place.
//
// observers may be nil, in which case every receiver is treated as near-side
// and this behaves exactly as it did before far-side receivers existed.
func DetectSegments(st *store.Store, nodes []store.Node, links []store.BridgeLink, observers []store.Observer, sinceISO string, scanCap int) (*SegmentReport, error) {
	rep := &SegmentReport{Members: []store.SegmentMember{}, Rejected: map[string]string{}}
	if len(links) == 0 {
		return rep, nil
	}
	if scanCap <= 0 || scanCap > 500000 {
		scanCap = 250000
	}
	raws, err := st.RawWindow(sinceISO, scanCap)
	if err != nil {
		return nil, err
	}

	// Sort the receivers: which of them sit on a bridge's far segment?
	//
	// The operator's declared far-side config is the reference. An observer
	// reporting a matching channel/bandwidth/spreading factor can hear that
	// segment, so its direct receptions are ground truth about membership; every
	// other observer — including one whose radio is unknown — stays near-side,
	// which is the conservative reading. A bridge with no declared far radio
	// classifies nobody, and its nodes fall through to the inferred rule.
	farObs, nFar := farObserverSets(links, observers)
	rep.FarObservers = nFar

	// A 1-byte far end is only usable when no other known node shares that byte.
	farByteUnique := map[string]bool{}
	for _, l := range links {
		b := l.Far[:2]
		n := 0
		for _, nd := range nodes {
			if strings.HasPrefix(strings.ToUpper(nd.PublicKey), b) {
				n++
			}
		}
		farByteUnique[l.Near] = n <= 1
	}

	// stats[bridgeNear][originKey]
	stats := map[string]map[string]*segStat{}
	for _, l := range links {
		stats[l.Near] = map[string]*segStat{}
	}

	for _, r := range raws {
		rep.Scanned++
		pkt, err := meshcore.DecodeHex(r.RawHex)
		if err != nil || pkt == nil || pkt.Advert == nil || pkt.Advert.PublicKey == "" {
			continue
		}
		origin := strings.ToUpper(pkt.Advert.PublicKey)
		path := pkt.RelayPath()

		for _, l := range links {
			// The bridge's own ends are not beyond it.
			if origin == l.Near || origin == l.Far {
				continue
			}
			s := stats[l.Near][origin]
			if s == nil {
				s = &segStat{}
				stats[l.Near][origin] = s
			}
			onFarSide := farObs[l.Near][r.ObserverID]
			if len(path) == 0 {
				// Same event, opposite meanings, decided entirely by which side
				// the receiver is on.
				if onFarSide {
					s.zeroHopFar++
				} else {
					s.zeroHop++
				}
				continue
			}
			if onFarSide {
				// A far-side receiver hearing relayed traffic says nothing about
				// which side the origin is on — the far segment carries both its
				// own nodes and everything that crossed the bridge into it. Left
				// out of the ratio entirely rather than counted as a near-side
				// sighting that never crossed, which would drag every far-side
				// node under the share bar.
				continue
			}
			s.withPath++

			iNearW, iFarW := -1, -1 // >=2-byte positions
			iNear1, iFar1 := -1, -1 // 1-byte positions
			for i, hop := range path {
				h := strings.ToUpper(hop)
				if h == "" {
					continue
				}
				wide := len(h)/2 >= 2
				if strings.HasPrefix(l.Near, h) {
					if wide && iNearW < 0 {
						iNearW = i
					} else if !wide && iNear1 < 0 {
						iNear1 = i
					}
				}
				if strings.HasPrefix(l.Far, h) {
					if wide && iFarW < 0 {
						iFarW = i
					} else if !wide && iFar1 < 0 {
						iFar1 = i
					}
				}
			}
			switch {
			case iNearW >= 0 && iFarW >= 0 && iNearW < iFarW:
				s.confirmed++
				rep.Crossings++
			case iNearW >= 0 && iFarW >= 0 && iFarW < iNearW:
				rep.Reverse++
			case farByteUnique[l.Near] && iNear1 >= 0 && iFar1 >= 0 && iNear1 < iFar1:
				s.probable++
				rep.Crossings++
			}
		}
	}

	names := map[string]string{}
	for _, n := range nodes {
		names[strings.ToUpper(n.PublicKey)] = n.Name
	}
	for _, l := range links {
		for origin, s := range stats[l.Near] {
			conf, reject := s.verdict()
			if conf == "" && reject == "" {
				continue // never crossed, never heard over there: not a candidate
			}
			label := names[origin]
			if label == "" {
				label = origin[:min(12, len(origin))]
			}
			if reject != "" {
				rep.Rejected[label] = reject
				continue
			}
			if conf == "observed" {
				rep.Observed++
			}
			rep.Members = append(rep.Members, store.SegmentMember{
				NodeKey: origin, BridgeNear: l.Near, Confidence: conf,
			})
		}
	}
	return rep, nil
}
