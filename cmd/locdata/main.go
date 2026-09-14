// Command locdata extracts the raw material for estimating where an
// unpositioned node is: for every stored advert, who heard it, at how many
// hops, with what SNR, and — the useful part — which relay carried it first.
//
// None of that is queryable as it stands. observations has no origin column:
// the sending node is recovered by decoding raw_hex, the same way cmd/orphans
// establishes what evidences a node. The relay path is in there too, and
// Path[0] is the one hop that demodulated the origin off the air, so it is the
// only hop whose position constrains where the origin can be.
//
// Output is a CSV for offline analysis; this tool computes no estimate itself.
// What the data says so far, measured over ~500k decoded adverts:
//
//   - SNR is useless for range, and worse than useless: it correlates POSITIVELY
//     with distance (r=+0.18), median +11.8 dB beyond 100 km against -2.0 dB at
//     5-10 km. Long links only exist between mountaintops with clean line of
//     sight, so conditioning on "heard at all" selects for good paths. Anything
//     inferring distance from SNR will place far nodes near.
//   - The SNR stored here is measured at the OBSERVER, after relaying. It says
//     nothing about the origin-to-first-hop link, and MeshCore carries no
//     per-hop SNR, so first hop gives connectivity only.
//   - First-hop relays localise roughly 3x better than observers, there being
//     ~279 located nodes against ~11 located observers.
//   - A path hash names a node only ambiguously (1 byte 28% of the time here).
//     Splitting weight across the candidates AVERAGES mutually exclusive
//     branches into a point that no evidence supports; picking the
//     geographically consistent reading instead took leave-one-out median error
//     from 10.8 km to 7.3 km.
//   - Even then the estimate collapses toward whichever high site relays most,
//     which is where the signal was RECEIVED, not where the node is. Treat the
//     output as "within range of these relays", not as a pin.
//   - Before using silence as evidence, filter by band: a node on 909 not
//     relaying for a node on 910.425 is a frequency difference, not distance.
//
// Usage:
//
//	go run ./cmd/locdata -db data/ridgeline.db -out /tmp/locdata.csv
package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

func main() {
	db := flag.String("db", "data/ridgeline.db", "path to ridgeline.db")
	out := flag.String("out", "/tmp/xloc.csv", "csv output")
	flag.Parse()

	conn, err := sql.Open("sqlite", "file:"+*db+"?mode=ro")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	rows, err := conn.Query(`SELECT message_hash, observer_id, path_hops, snr, rssi, received_at, raw_hex FROM observations`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	f, err := os.Create(*out)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"origin", "observer", "hops", "snr", "rssi", "received_at", "msg_hash", "path0", "hashsize", "pathlen"})

	var scanned, decoded int
	for rows.Next() {
		var mh, obs, recv, raw string
		var hops sql.NullInt64
		var snr, rssi sql.NullFloat64
		if err := rows.Scan(&mh, &obs, &hops, &snr, &rssi, &recv, &raw); err != nil {
			panic(err)
		}
		scanned++
		if scanned%500000 == 0 {
			fmt.Fprintf(os.Stderr, "  scanned %d…\n", scanned)
		}
		pkt, err := meshcore.DecodeHex(raw)
		if err != nil || pkt == nil || pkt.Advert == nil || pkt.Advert.PublicKey == "" {
			continue
		}
		decoded++
		fs := func(v sql.NullFloat64) string {
			if !v.Valid {
				return ""
			}
			return strconv.FormatFloat(v.Float64, 'f', 1, 64)
		}
		h := ""
		if hops.Valid {
			h = strconv.FormatInt(hops.Int64, 10)
		}
		// Path[0] is the relay that demodulated the origin off the air, so it is
		// the one hop whose position constrains where the origin can be.
		path0 := ""
		if len(pkt.Path) > 0 {
			path0 = strings.ToUpper(pkt.Path[0])
		}
		w.Write([]string{strings.ToUpper(pkt.Advert.PublicKey), strings.ToUpper(obs), h, fs(snr), fs(rssi), recv, mh,
			path0, strconv.Itoa(pkt.PathHashSize), strconv.Itoa(len(pkt.Path))})
	}
	fmt.Fprintf(os.Stderr, "scanned=%d adverts_decoded=%d\n", scanned, decoded)
}
