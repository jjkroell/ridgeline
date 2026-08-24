// Command segprobe answers one question: can traffic that originates on the
// NEAR segment cross the bridge and be heard by a FAR-side observer with no
// relay path, which would forge the exact evidence the sweep treats as proof
// that a node lives on the far side?
//
// It replays the sweep's own inputs and, for every origin the far observer
// heard pathlessly, prints how that same origin looks to near-side receivers.
// A node heard directly on BOTH sides is a contradiction; a node heard
// pathlessly ONLY by the far observer is what "observed" rests on.
//
// Throwaway diagnostic — not part of the daemon.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/radio"
	"github.com/jjkroell/ridgeline/internal/store"
)

type stat struct {
	zeroHopFar, zeroHopNear int // pathless, by which side heard it
	pathFar, pathNear       int // carried a relay path
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: segprobe <db> <sinceHours>")
		os.Exit(1)
	}
	dbPath := os.Args[1]
	var hours int
	fmt.Sscan(os.Args[2], &hours)
	if len(os.Args) > 3 && os.Args[3] == "dryrun" {
		dryRun(dbPath, hours)
		return
	}

	st, err := store.Open(dbPath)
	must(err)
	defer st.Close()

	links, err := st.KnownBridgeLinks()
	must(err)
	if len(links) == 0 {
		fmt.Println("no sanctioned bridges")
		return
	}
	obs, err := st.ListObservers()
	must(err)
	nodes, err := st.ListNodes()
	must(err)

	name := map[string]string{}
	nodeRadio := map[string]string{}
	for _, n := range nodes {
		k := strings.ToUpper(n.PublicKey)
		name[k] = n.Name
		nodeRadio[k] = n.Radio
	}

	l := links[0]
	fmt.Printf("bridge %q  near=%s far=%s  declared far radio=%q\n\n",
		l.Name, short(l.NearEnd()), short(l.FarEnd()), l.PeerRadio)

	// Far-side observers, by the same numeric rule the sweep uses.
	farObs := map[string]bool{}
	for _, o := range obs {
		if o.Radio != "" && l.PeerRadio != "" && radio.SameSegmentString(o.Radio, l.PeerRadio) {
			farObs[o.ID] = true
			fmt.Printf("FAR observer: %-28s radio=%s\n", o.Name, o.Radio)
		}
	}
	fmt.Println()

	since := time.Now().Add(-time.Duration(hours) * time.Hour).UTC().Format(time.RFC3339Nano)
	raws, err := st.RawWindow(since, 5000000)
	must(err)

	stats := map[string]*stat{}
	for _, r := range raws {
		pkt, err := meshcore.DecodeHex(r.RawHex)
		if err != nil || pkt == nil || pkt.Advert == nil || pkt.Advert.PublicKey == "" {
			continue
		}
		origin := strings.ToUpper(pkt.Advert.PublicKey)
		s := stats[origin]
		if s == nil {
			s = &stat{}
			stats[origin] = s
		}
		pathless := len(pkt.RelayPath()) == 0
		switch {
		case farObs[r.ObserverID] && pathless:
			s.zeroHopFar++
		case farObs[r.ObserverID]:
			s.pathFar++
		case pathless:
			s.zeroHopNear++
		default:
			s.pathNear++
		}
	}

	type row struct {
		key string
		s   *stat
	}
	var rows []row
	for k, s := range stats {
		if s.zeroHopFar > 0 {
			rows = append(rows, row{k, s})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].s.zeroHopFar > rows[j].s.zeroHopFar })

	// The load-bearing claim: crossed traffic ARRIVES WITH A PATH. Quantify it
	// over exactly the window that was scanned, since the query is capped.
	var farTotal, farPathless int
	first, last := "", ""
	for _, r := range raws {
		if r.ReceivedAt != "" {
			if first == "" || r.ReceivedAt < first {
				first = r.ReceivedAt
			}
			if r.ReceivedAt > last {
				last = r.ReceivedAt
			}
		}
		if !farObs[r.ObserverID] {
			continue
		}
		farTotal++
		pkt, err := meshcore.DecodeHex(r.RawHex)
		if err != nil || pkt == nil {
			continue
		}
		if len(pkt.RelayPath()) == 0 {
			farPathless++
		}
	}
	fmt.Printf("SCAN WINDOW ACTUALLY COVERED: %s .. %s (%d raws)\n", first, last, len(raws))
	fmt.Printf("far observer heard %d packets; %d were PATHLESS (%.2f%%), %d carried a relay path\n\n",
		farTotal, farPathless, 100*float64(farPathless)/float64(max(farTotal, 1)), farTotal-farPathless)

	// ---- FIRST-HOP INFERENCE FEASIBILITY ----
	// path[0] is the relay that heard the origin OVER THE AIR (crossingKind
	// relies on this ordering). If that relay is known to sit on the far
	// segment, the origin must have transmitted there. Whether that is usable
	// depends entirely on how many BYTES the path carries: a 1-byte hash rarely
	// names one node.
	widths := map[int]int{}
	firstHopResolved, firstHopAmbiguous, firstHopUnknown := 0, 0, 0
	var withPath int
	for _, r := range raws {
		pkt, err := meshcore.DecodeHex(r.RawHex)
		if err != nil || pkt == nil || pkt.Advert == nil || pkt.Advert.PublicKey == "" {
			continue
		}
		path := pkt.RelayPath()
		if len(path) == 0 {
			continue
		}
		withPath++
		h := strings.ToUpper(path[0])
		widths[len(h)/2]++
		n := 0
		for k := range name {
			if strings.HasPrefix(k, h) {
				n++
			}
		}
		switch {
		case n == 1:
			firstHopResolved++
		case n > 1:
			firstHopAmbiguous++
		default:
			firstHopUnknown++
		}
	}
	fmt.Printf("FIRST-HOP RESOLVABILITY over %d adverts that carried a path\n", withPath)
	ws := []int{}
	for w := range widths {
		ws = append(ws, w)
	}
	sort.Ints(ws)
	for _, w := range ws {
		fmt.Printf("  path[0] width %d byte(s): %6d (%.1f%%)\n", w, widths[w], 100*float64(widths[w])/float64(max(withPath,1)))
	}
	fmt.Printf("  resolves to exactly ONE known node: %d (%.1f%%)\n", firstHopResolved, 100*float64(firstHopResolved)/float64(max(withPath,1)))
	fmt.Printf("  matches SEVERAL known nodes:        %d (%.1f%%)\n", firstHopAmbiguous, 100*float64(firstHopAmbiguous)/float64(max(withPath,1)))
	fmt.Printf("  matches NO known node:              %d (%.1f%%)\n\n", firstHopUnknown, 100*float64(firstHopUnknown)/float64(max(withPath,1)))

	fmt.Printf("EVERY origin the far observer heard PATHLESSLY (scanned %d raws, %dh)\n", len(raws), hours)
	fmt.Printf("%-26s %8s %9s %8s %9s  %-18s %s\n",
		"node", "0hopFAR", "0hopNEAR", "pathFAR", "pathNEAR", "stored radio", "verdict")
	for _, r := range rows {
		v := "far-side only (clean)"
		if r.s.zeroHopNear > 0 {
			v = "!! HEARD DIRECTLY ON BOTH SIDES"
		}
		if r.key == l.NearEnd() || r.key == l.FarEnd() {
			v += " [a bridge end]"
		}
		fmt.Printf("%-26s %8d %9d %8d %9d  %-18s %s\n",
			trunc(nm(name, r.key), 26), r.s.zeroHopFar, r.s.zeroHopNear, r.s.pathFar, r.s.pathNear,
			orDash(nodeRadio[r.key]), v)
	}
}

func nm(m map[string]string, k string) string {
	if m[k] != "" {
		return m[k]
	}
	return short(k)
}
func short(k string) string {
	if len(k) > 8 {
		return k[:8]
	}
	return k
}
func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
func orDash(s string) string {
	if s == "" {
		return "(null)"
	}
	return s
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
