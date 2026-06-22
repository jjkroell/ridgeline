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

	apiServer := api.New(st, log, version, cfg.WebDir)

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
	go runRetention(ctx, st, log)

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
