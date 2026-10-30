// Command purgechannel removes every stored message of one hashtag channel —
// all observations whose GroupText decrypts to it — e.g. to clear #sos of its
// set-up chatter. Only the packets go: the channel's monitor roster
// (channel_monitors) and everything else are kept. Writes a JSON backup of
// the rows it deletes first.
//
//	purgechannel -db data/ridgeline.db -channel sos                 # dry run
//	purgechannel -db data/ridgeline.db -channel sos -apply -backup data/sos-purged.json
//
// Safe beside a running daemon: the scan reads by the hour and the delete runs
// in small batches. The daemon's /api/stats count catches up at its daily
// recount; restart it to drop an in-memory #sos alert flag the purge removed
// the message for. Run with the daemon's file permissions so the WAL can be written.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

func main() {
	dbPath := flag.String("db", "data/ridgeline.db", "database path")
	channel := flag.String("channel", "", "hashtag channel name, with or without '#' (case-sensitive)")
	days := flag.Int("days", 60, "how far back to look")
	backup := flag.String("backup", "", "write the deleted rows here as JSON (required with -apply)")
	apply := flag.Bool("apply", false, "actually delete (default is a dry run)")
	hashes := flag.String("hash", "", "only these message hashes (comma-separated; default: the whole channel)")
	flag.Parse()
	name := strings.TrimPrefix(*channel, "#")
	if name == "" {
		fmt.Fprintln(os.Stderr, "set -channel (e.g. sos)")
		os.Exit(2)
	}
	if *apply && *backup == "" {
		fmt.Fprintln(os.Stderr, "-apply needs -backup <file>")
		os.Exit(2)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer st.Close()
	meshcore.RegisterChannel(name, meshcore.DeriveHashtagKey(name))

	t0 := time.Now()
	obs, err := st.ChannelObservations(context.Background(), name, *days, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan:", err)
		os.Exit(1)
	}
	if *hashes != "" {
		keep := map[string]bool{}
		for _, h := range strings.Split(*hashes, ",") {
			keep[strings.TrimSpace(h)] = true
		}
		sel := obs[:0]
		for _, o := range obs {
			if keep[o.MessageHash] {
				sel = append(sel, o)
			}
		}
		obs = sel
	}
	msgs := map[string]store.ChannelObservation{}
	for _, o := range obs {
		if ex, ok := msgs[o.MessageHash]; !ok || o.ReceivedAt < ex.ReceivedAt {
			msgs[o.MessageHash] = o
		}
	}
	for _, o := range msgs {
		fmt.Printf("%s  %s  %-24.24s %s\n", o.ReceivedAt[:19], o.MessageHash, o.Sender, o.Message)
	}
	fmt.Printf("#%s: %d message(s), %d observation(s) in the last %d days (scan %s)\n",
		name, len(msgs), len(obs), *days, time.Since(t0).Round(time.Millisecond))
	if !*apply || len(obs) == 0 {
		fmt.Println("dry run — nothing deleted (add -apply -backup <file>)")
		return
	}

	b, _ := json.MarshalIndent(obs, "", "  ")
	if err := os.WriteFile(*backup, b, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "backup:", err)
		os.Exit(1)
	}
	ids := make([]int64, len(obs))
	for i, o := range obs {
		ids[i] = o.ID
	}
	n, err := st.DeleteObservations(ids)
	if err != nil {
		fmt.Fprintln(os.Stderr, "delete:", err)
		os.Exit(1)
	}
	fmt.Printf("deleted %d observation(s); backup in %s; monitor roster kept\n", n, *backup)
}
