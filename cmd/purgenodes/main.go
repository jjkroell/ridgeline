// Command purgenodes removes nodes located east of a given longitude, the same
// way the retention sweep removes a silent node: the node row and its stored
// adverts go, while claims, notes and private locations are KEPT (they reattach
// if the node is heard again). Nodes at 0,0 — GPS never set — are skipped,
// since "east" is meaningless for them.
//
//	purgenodes -db data/ridgeline.db -east-of -122.47            # dry run
//	purgenodes -db data/ridgeline.db -east-of -122.47 -apply
//
// Safe beside a running daemon: the purge works in short batches and releases
// the database between them (see store.purgeTargets). Run with the daemon's
// file permissions so the WAL can be written.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

func main() {
	dbPath := flag.String("db", "data/ridgeline.db", "database path")
	eastOf := flag.Float64("east-of", 0, "remove located nodes whose longitude is greater than this (e.g. -122.47)")
	apply := flag.Bool("apply", false, "actually delete (default is a dry run)")
	flag.Parse()
	if *eastOf == 0 {
		fmt.Fprintln(os.Stderr, "set -east-of (a longitude, e.g. -122.47)")
		os.Exit(2)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer st.Close()

	nodes, err := st.ListNodes()
	if err != nil {
		fmt.Fprintln(os.Stderr, "list nodes:", err)
		os.Exit(1)
	}
	var keys []string
	var lines []string
	for _, n := range nodes {
		if !n.HasLocation || n.Latitude == nil || n.Longitude == nil {
			continue
		}
		if *n.Latitude == 0 && *n.Longitude == 0 {
			continue // GPS never set
		}
		if *n.Longitude <= *eastOf {
			continue
		}
		keys = append(keys, strings.ToUpper(n.PublicKey))
		lines = append(lines, fmt.Sprintf("%9.3f %8.3f  %s  %s", *n.Longitude, *n.Latitude, n.PublicKey[:10], n.Name))
	}
	sort.Strings(lines)
	for _, l := range lines {
		fmt.Println(l)
	}
	fmt.Printf("%d node(s) east of %.3f\n", len(keys), *eastOf)
	if !*apply || len(keys) == 0 {
		fmt.Println("dry run — nothing deleted (add -apply)")
		return
	}

	t0 := time.Now()
	res, err := st.PurgeTargets(nil, nil, keys)
	if err != nil {
		fmt.Fprintln(os.Stderr, "purge:", err)
		os.Exit(1)
	}
	fmt.Printf("deleted %d node row(s) and %d observation(s) in %s; user data kept\n",
		res.Nodes, res.Observations, time.Since(t0).Round(time.Millisecond))
}
