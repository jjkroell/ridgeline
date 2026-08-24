package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/analytics"
	"github.com/jjkroell/ridgeline/internal/store"
)

// dryRun replays the real sweep against a database copy and prints what it
// WOULD write, without applying anything.
func dryRun(dbPath string, hours int) {
	st, err := store.Open(dbPath)
	must(err)
	defer st.Close()

	nodes, err := st.ListNodes()
	must(err)
	links, err := st.KnownBridgeLinks()
	must(err)
	obs, err := st.ListObservers()
	must(err)

	name := map[string]string{}
	for _, n := range nodes {
		name[strings.ToUpper(n.PublicKey)] = n.Name
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour).UTC().Format(time.RFC3339Nano)
	rep, err := analytics.DetectSegments(st, nodes, links, obs, since, 200000)
	must(err)

	fmt.Printf("scanned=%d crossings=%d reverse=%d far_observers=%d observed=%d members=%d measured=%d\n\n",
		rep.Scanned, rep.Crossings, rep.Reverse, rep.FarObservers, rep.Observed,
		len(rep.Members), len(rep.MeasuredRadio))

	sort.Slice(rep.Members, func(i, j int) bool {
		return name[strings.ToUpper(rep.Members[i].NodeKey)] < name[strings.ToUpper(rep.Members[j].NodeKey)]
	})
	fmt.Printf("%-28s %-11s %s\n", "member", "confidence", "radio it would be given")
	for _, m := range rep.Members {
		k := strings.ToUpper(m.NodeKey)
		r := rep.MeasuredRadio[k]
		if r == "" {
			r = "— (declared value stands in)"
		} else {
			r = "MEASURED " + r
		}
		fmt.Printf("%-28s %-11s %s\n", trunc(nmOr(name, k), 28), m.Confidence, r)
	}
	if len(rep.Rejected) > 0 {
		fmt.Printf("\nrejected:\n")
		for n, why := range rep.Rejected {
			fmt.Printf("  %-26s %s\n", trunc(n, 26), why)
		}
	}
	if len(rep.ReversedEnds) > 0 {
		fmt.Printf("\n!! reversed ends: %v\n", rep.ReversedEnds)
	}
}

func nmOr(m map[string]string, k string) string {
	if m[k] != "" {
		return m[k]
	}
	return short(k)
}
