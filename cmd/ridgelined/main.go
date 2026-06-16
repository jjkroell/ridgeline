// Command ridgelined is the Ridgeline daemon: it ingests MeshCore packets
// from MQTT, decodes them into SQLite, and serves the REST API, WebSocket
// live feed, and the built web UI.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
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
	log.Info("ridgelined starting", "version", version, "config", *configPath)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","version":%q}`, version)
	})

	addr := ":8080"
	log.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error("server exited", "err", err)
		os.Exit(1)
	}
}
