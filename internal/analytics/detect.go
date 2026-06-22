package analytics

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// Detection thresholds. Deliberately conservative — these surface *candidates*
// for an admin to confirm, not auto-bans.
const (
	minForeignThrough = 3   // foreign nodes entering via a node, to flag it a bridge
	minBridgeSpecific = 0.8 // fraction of through-traffic that must be foreign (a real
	// bridge is ~all foreign; legit hubs carry mixed traffic)
	minExclusiveNodes = 3 // nodes sourced by only one observer, to flag an injector
)

// InjectionReport lists detected ingress points for foreign/injected traffic.
type InjectionReport struct {
	WindowHours float64             `json:"windowHours"`
	Bridges     []BridgeCandidate   `json:"bridges"`   // RF bridges
	Injectors   []InjectorCandidate `json:"injectors"` // rogue MQTT publishers
}

// ForeignNode is a node identified as injected (heard only via a bridge/injector).
type ForeignNode struct {
	Key       string   `json:"key"`
	Name      string   `json:"name"`
	Role      string   `json:"role,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

// BridgeCandidate is a node through which a population of never-directly-heard
// nodes enters the mesh — the RF-bridge signature.
type BridgeCandidate struct {
	NodeKey      string  `json:"nodeKey"`
	Name         string  `json:"name"`
	ForeignCount int     `json:"foreignCount"` // distinct foreign origins entering via it
	ThroughTotal int     `json:"throughTotal"` // all origins routed through it
	Specificity  float64 `json:"specificity"`  // foreign / through (1.0 = only foreign)
	// ForeignKm is how far the foreign set's geographic centroid sits from the
	// mesh centroid. A cross-mesh bridge imports a geographically displaced
	// cluster (the strongest corroborator); 0 when locations are unknown.
	ForeignKm float64       `json:"foreignKm"`
	Foreign   []ForeignNode `json:"foreign"`
}

// InjectorCandidate is an observer that is the sole source of a population of
// nodes — the rogue-MQTT-publisher signature.
type InjectorCandidate struct {
	Observer       string        `json:"observer"`
	ExclusiveCount int           `json:"exclusiveCount"`
	Exclusive      []ForeignNode `json:"exclusive"`
}

// DetectInjection scans a window of observations and flags likely ingress points
// for foreign traffic, by two independent signatures:
//
//   - RF bridge: a node that sits on the flood path of many origins that are
//     never heard zero-hop by any observer, and whose through-traffic is mostly
//     such foreign origins (high specificity vs. a legitimate hub relay).
//   - MQTT injector: an observer that is the *only* source of many origins no
//     other observer ever reports.
//
// Origin classification keys on adverts (the only packets whose origin is in the
// clear); that is exactly the foreign-node population an admin wants to remove.
func DetectInjection(st *store.Store, nodes []store.Node, sinceISO string, scanCap int) (*InjectionReport, error) {
	if scanCap <= 0 || scanCap > 300000 {
		scanCap = 120000
	}
	raws, err := st.RawWindow(sinceISO, scanCap)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]store.Node, len(nodes))
	for _, n := range nodes {
		byKey[strings.ToUpper(n.PublicKey)] = n
	}
	resolve := newPrefixResolver(nodes)

	directlyHeard := map[string]bool{}                // origin heard at zero hops
	reporters := map[string]map[string]bool{}         // origin -> set of observer ids
	txHops := map[string]map[string]map[string]bool{} // origin -> msgHash -> set of resolved relay keys

	for _, ro := range raws {
		pkt, err := meshcore.DecodeHex(ro.RawHex)
		if err != nil || pkt == nil || pkt.Advert == nil || pkt.Advert.PublicKey == "" {
			continue
		}
		origin := strings.ToUpper(pkt.Advert.PublicKey)
		if ro.ObserverID != "" {
			if reporters[origin] == nil {
				reporters[origin] = map[string]bool{}
			}
			reporters[origin][ro.ObserverID] = true
		}
		if len(pkt.Path) == 0 {
			directlyHeard[origin] = true
			continue
		}
		if txHops[origin] == nil {
			txHops[origin] = map[string]map[string]bool{}
		}
		hops := txHops[origin][pkt.MessageHash]
		if hops == nil {
			hops = map[string]bool{}
			txHops[origin][pkt.MessageHash] = hops
		}
		for _, h := range pkt.Path {
			if k := resolve(h); k != "" {
				hops[strings.ToUpper(k)] = true
			}
		}
	}

	relayOnly := func(origin string) bool { return !directlyHeard[origin] }

	// For each origin, the relays present in EVERY one of its transmissions.
	through := map[string]map[string]bool{}        // relay -> set of origins routed through it
	foreignThrough := map[string]map[string]bool{} // relay -> set of foreign origins
	for origin, txs := range txHops {
		var always map[string]bool
		for _, hops := range txs {
			if always == nil {
				always = map[string]bool{}
				for k := range hops {
					always[k] = true
				}
				continue
			}
			for k := range always {
				if !hops[k] {
					delete(always, k)
				}
			}
		}
		for r := range always {
			if through[r] == nil {
				through[r] = map[string]bool{}
				foreignThrough[r] = map[string]bool{}
			}
			through[r][origin] = true
			if relayOnly(origin) {
				foreignThrough[r][origin] = true
			}
		}
	}

	report := &InjectionReport{WindowHours: windowHoursFrom(sinceISO)}
	meshLat, meshLon, haveMesh := centroid(nodes)

	// Bridge candidates.
	for r, fset := range foreignThrough {
		if len(fset) < minForeignThrough {
			continue
		}
		tot := len(through[r])
		spec := float64(len(fset)) / float64(max(1, tot))
		if spec < minBridgeSpecific {
			continue // a legitimate hub relays mostly local traffic
		}
		foreign := foreignNodes(fset, byKey)
		bc := BridgeCandidate{
			NodeKey:      r,
			Name:         displayName(byKey[r], r),
			ForeignCount: len(fset),
			ThroughTotal: tot,
			Specificity:  spec,
			Foreign:      foreign,
		}
		if haveMesh {
			if fLat, fLon, ok := foreignCentroid(foreign); ok {
				bc.ForeignKm = haversineKm(meshLat, meshLon, fLat, fLon)
			}
		}
		report.Bridges = append(report.Bridges, bc)
	}
	// Rank by a composite: more foreign origins, higher specificity, and greater
	// geographic displacement all raise suspicion of a real cross-mesh bridge.
	score := func(b BridgeCandidate) float64 {
		return float64(b.ForeignCount) * b.Specificity * (1 + b.ForeignKm/50)
	}
	sort.Slice(report.Bridges, func(i, j int) bool {
		return score(report.Bridges[i]) > score(report.Bridges[j])
	})

	// MQTT injector candidates: observers that are the sole source of foreign nodes.
	exclusive := map[string]map[string]bool{} // observer -> origins only it reports
	for origin, reps := range reporters {
		if len(reps) != 1 || !relayOnly(origin) {
			continue
		}
		var only string
		for o := range reps {
			only = o
		}
		if exclusive[only] == nil {
			exclusive[only] = map[string]bool{}
		}
		exclusive[only][origin] = true
	}
	for obs, set := range exclusive {
		if len(set) < minExclusiveNodes {
			continue
		}
		report.Injectors = append(report.Injectors, InjectorCandidate{
			Observer:       obs,
			ExclusiveCount: len(set),
			Exclusive:      foreignNodes(set, byKey),
		})
	}
	sort.Slice(report.Injectors, func(i, j int) bool {
		return report.Injectors[i].ExclusiveCount > report.Injectors[j].ExclusiveCount
	})

	return report, nil
}

func foreignNodes(set map[string]bool, byKey map[string]store.Node) []ForeignNode {
	out := make([]ForeignNode, 0, len(set))
	for k := range set {
		n := byKey[k]
		fn := ForeignNode{Key: k, Name: displayName(n, k), Role: n.Role}
		fn.Latitude, fn.Longitude = n.Latitude, n.Longitude
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// centroid returns the median lat/lon of all located nodes (median resists the
// pull of a displaced foreign cluster better than the mean).
func centroid(nodes []store.Node) (lat, lon float64, ok bool) {
	var lats, lons []float64
	for _, n := range nodes {
		if n.Latitude != nil && n.Longitude != nil && (*n.Latitude != 0 || *n.Longitude != 0) {
			lats = append(lats, *n.Latitude)
			lons = append(lons, *n.Longitude)
		}
	}
	if len(lats) == 0 {
		return 0, 0, false
	}
	return median(lats), median(lons), true
}

func foreignCentroid(fs []ForeignNode) (lat, lon float64, ok bool) {
	var lats, lons []float64
	for _, f := range fs {
		if f.Latitude != nil && f.Longitude != nil && (*f.Latitude != 0 || *f.Longitude != 0) {
			lats = append(lats, *f.Latitude)
			lons = append(lons, *f.Longitude)
		}
	}
	if len(lats) == 0 {
		return 0, 0, false
	}
	return median(lats), median(lons), true
}

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// haversineKm is the great-circle distance between two lat/lon points in km.
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func windowHoursFrom(sinceISO string) float64 {
	t := parseTime(sinceISO)
	if t.IsZero() {
		return 0
	}
	return time.Since(t).Hours()
}
