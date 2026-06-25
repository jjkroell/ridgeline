// Command ridgelined is the Ridgeline daemon: it ingests MeshCore packets
// from MQTT, decodes them into SQLite, and serves the REST API, WebSocket
// live feed, and the built web UI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jjkroell/ridgeline/internal/analytics"
	"github.com/jjkroell/ridgeline/internal/api"
	"github.com/jjkroell/ridgeline/internal/config"
	"github.com/jjkroell/ridgeline/internal/ingest"
	"github.com/jjkroell/ridgeline/internal/store"
)

var version = "dev" // set via -ldflags at build time

func main() {
	configPath := flag.String("config", "config.json", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("ridgelined", version)
		return
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log, *configPath); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		// Config file is optional in dev; fall back to defaults.
		log.Warn("using default config", "reason", err)
		cfg = config.Default()
	}
	log.Info("ridgelined starting", "version", version, "db", cfg.DBPath, "broker", cfg.MQTT.Broker)

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	// One-time seed of advert_tx_count (actual transmissions) from history when
	// the column was just added, before ingest starts touching it.
	if st.NeedsAdvertTxBackfill() {
		if n, txs, err := analytics.BackfillAdvertTx(st); err != nil {
			log.Warn("advert-tx backfill", "err", err)
		} else {
			log.Info("advert-tx backfill complete", "nodes", n, "transmissions", txs)
		}
	}

	apiServer := api.New(st, log, version, cfg.WebDir, cfg.AdminToken)

	// Per-node analytics snapshot, recomputed periodically over a rolling window.
	engine := analytics.New(6)
	apiServer.SetAnalytics(engine)

	in := ingest.New(cfg.MQTT, st, log)
	in.OnObservation = apiServer.Broadcast
	if err := in.Start(); err != nil {
		// Don't abort the whole daemon if the broker is briefly unavailable;
		// paho retries the connection in the background.
		log.Warn("mqtt connect pending", "err", err)
	}
	defer in.Stop()

	srv := &http.Server{Addr: cfg.ListenAddr, Handler: apiServer.Handler()}

	go func() {
		log.Info("listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server exited", "err", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go runAnalytics(ctx, engine, st, log)
	go runHashSizeConsensus(ctx, st, log)
	go runRetention(ctx, st, log)
	if cfg.NodeRetentionDays > 0 {
		go runNodeRetention(ctx, st, engine, cfg.NodeRetentionDays, log)
	}
	if cfg.ScrubArtifacts {
		go runArtifactScrub(ctx, st, log)
	}

	<-ctx.Done()

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// runAnalytics recomputes the per-node analytics snapshot immediately, then
// every 90s, until ctx is cancelled.
func runAnalytics(ctx context.Context, engine *analytics.Engine, st *store.Store, log *slog.Logger) {
	recompute := func() {
		nodes, err := st.ListNodes()
		if err != nil {
			log.Warn("analytics: list nodes", "err", err)
			return
		}
		if err := engine.Recompute(st, nodes); err != nil {
			log.Warn("analytics: recompute", "err", err)
		}
	}
	recompute()
	t := time.NewTicker(90 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			recompute()
		}
	}
}

// hashSizeConsensusWindow is how far back the hash-size vote looks. The vote is
// per-transmission, and nodes advert only about once a day (~30h observed), so
// the window must span several cadences to gather enough independent broadcasts
// for a confident majority — one corrupt transmission must be a clear minority.
// A week gives ~5 transmissions for a typical node while a quiet node simply
// stays untouched until it has spoken enough.
const hashSizeConsensusWindow = 7 * 24 * time.Hour

// hashSizeConsensusInterval is how often the vote re-runs. Frequent enough to
// repair a misread size promptly, cheap enough to scan the window each time.
const hashSizeConsensusInterval = 6 * time.Hour

// runHashSizeConsensus periodically repairs each node's stored hash size by
// majority vote over its recent adverts. Ingest sets hash_size only while it's
// unknown (a corrupt path-length byte can no longer flip an established size),
// but the very first advert seen for a node could itself be corrupt — so this
// pass owns corrections, voting over the window and overriding the stored value
// when a clear winner disagrees with it. Runs shortly after startup, then on a
// fixed interval.
func runHashSizeConsensus(ctx context.Context, st *store.Store, log *slog.Logger) {
	reconcile := func() {
		cutoff := time.Now().Add(-hashSizeConsensusWindow).UTC().Format(time.RFC3339Nano)
		consensus, err := analytics.ConsensusHashSizes(st, cutoff)
		if err != nil {
			log.Warn("hash-size consensus: compute", "err", err)
			return
		}
		nodes, err := st.ListNodes()
		if err != nil {
			log.Warn("hash-size consensus: list nodes", "err", err)
			return
		}
		current := make(map[string]int, len(nodes))
		for _, n := range nodes {
			current[n.PublicKey] = n.HashSize
		}
		// Only write where the verdict differs from what's stored (also fills an
		// unknown 0). Keeps the update small and the log meaningful.
		corrections := map[string]int{}
		for pk, size := range consensus {
			if cur, ok := current[pk]; ok && cur != size {
				corrections[pk] = size
			}
		}
		if len(corrections) == 0 {
			return
		}
		if err := st.SetHashSizes(corrections); err != nil {
			log.Warn("hash-size consensus: update", "err", err)
			return
		}
		log.Info("hash-size consensus: corrected node hash sizes", "nodes", len(corrections))
	}

	// Let ingest settle so the window holds recent adverts before the first vote.
	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Minute):
	}
	reconcile()
	t := time.NewTicker(hashSizeConsensusInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			reconcile()
		}
	}
}

// telemetryRetention is how long observer telemetry samples are kept. Two weeks
// leaves room for week-over-week comparison while bounding table growth (~12
// observers × one sample / 5 min ≈ a few thousand rows/day).
const telemetryRetention = 14 * 24 * time.Hour

// runRetention prunes aged observer-telemetry samples immediately, then daily,
// until ctx is cancelled.
func runRetention(ctx context.Context, st *store.Store, log *slog.Logger) {
	prune := func() {
		before := time.Now().Add(-telemetryRetention).UTC().Format(time.RFC3339Nano)
		n, err := st.PruneTelemetry(before)
		if err != nil {
			log.Warn("retention: prune telemetry", "err", err)
			return
		}
		if n > 0 {
			log.Info("retention: pruned telemetry", "rows", n)
		}
	}
	prune()
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			prune()
		}
	}
}

// runNodeRetention removes nodes that have gone silent past the retention
// threshold — no advert for retentionDays, and not currently relaying (the
// analytics liveness snapshot guards still-active relays whose advert is stale).
// A pruned node's row and stored adverts go; it reappears the moment it
// transmits again, so this only clears genuinely-departed nodes. Like the
// artifact sweep it waits for ingest/analytics to settle, then runs daily.
func runNodeRetention(ctx context.Context, st *store.Store, engine *analytics.Engine, retentionDays int, log *slog.Logger) {
	prune := func() {
		nodes, err := st.ListNodes()
		if err != nil {
			log.Warn("node retention: list nodes", "err", err)
			return
		}
		cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
		keys := analytics.StaleNodeKeys(nodes, engine.Liveness(), cutoff)
		if len(keys) == 0 {
			return
		}
		res, err := st.PurgeTargets(nil, nil, keys)
		if err != nil {
			log.Warn("node retention: purge", "err", err)
			return
		}
		log.Info("node retention: removed silent nodes",
			"thresholdDays", retentionDays, "candidates", len(keys),
			"nodesDeleted", res.Nodes, "observationsDeleted", res.Observations)
	}

	// Let ingest/analytics settle so liveness reflects current relays before the
	// first sweep (a cold snapshot would under-protect active relays).
	select {
	case <-ctx.Done():
		return
	case <-time.After(3 * time.Minute):
	}
	prune()
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			prune()
		}
	}
}

// artifactScrubInterval is how often the corruption-artifact sweep runs. Daily is
// plenty — artifacts are harmless until removed, and only high-confidence
// (provably corrupt) records are ever deleted.
const artifactScrubInterval = 24 * time.Hour

// runArtifactScrub periodically removes high-confidence packet-corruption
// artifacts — phantom node records whose public key arrived with bytes flipped,
// the same records surfaced in the Hash-IDs UI. It deletes the node row + its
// observations with NO blocklist entry: the exact corrupt key is random and
// unlikely to recur, and if it does the next sweep catches it. Runs a couple of
// minutes after startup, then every 24h.
func runArtifactScrub(ctx context.Context, st *store.Store, log *slog.Logger) {
	scrub := func() {
		nodes, err := st.ListNodes()
		if err != nil {
			log.Warn("artifact scrub: list nodes", "err", err)
			return
		}
		keys := analytics.HighConfidenceArtifactKeys(nodes)
		if len(keys) == 0 {
			return
		}
		res, err := st.PurgeTargets(nil, nil, keys)
		if err != nil {
			log.Warn("artifact scrub: purge", "err", err)
			return
		}
		log.Info("artifact scrub: removed corruption artifacts",
			"candidates", len(keys), "nodesDeleted", res.Nodes, "observationsDeleted", res.Observations)
	}

	// Let ingest/analytics settle before the first sweep.
	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Minute):
	}
	scrub()
	t := time.NewTicker(artifactScrubInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			scrub()
		}
	}
}
