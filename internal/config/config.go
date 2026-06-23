// Package config loads ridgelined's runtime configuration from a JSON file.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config is the daemon's runtime configuration.
type Config struct {
	// ListenAddr is the host:port the HTTP/WebSocket server binds to.
	ListenAddr string `json:"listenAddr"`
	// DBPath is the SQLite database file path.
	DBPath string `json:"dbPath"`
	// WebDir is the directory of built static web assets to serve. Empty
	// disables static serving (API only).
	WebDir string `json:"webDir"`
	// MQTT configures the upstream packet source.
	MQTT MQTT `json:"mqtt"`
	// AdminToken gates the /api/admin/* endpoints (detection, quarantine,
	// purge). The admin panel sends it as a Bearer token. Empty disables the
	// admin API entirely (safe default). Serve over TLS — this token grants
	// destructive powers.
	AdminToken string `json:"adminToken"`
	// ScrubArtifacts enables the periodic auto-removal of packet-corruption
	// artifacts (phantom node records whose public key arrived corrupted). Only
	// high-confidence artifacts are ever deleted. Defaults to true; set false to
	// disable the sweep.
	ScrubArtifacts bool `json:"scrubArtifacts"`
}

// MQTT configures the connection to a MeshCore observer broker.
type MQTT struct {
	Broker   string   `json:"broker"`   // e.g. tcp://host:1883
	ClientID string   `json:"clientID"` // MQTT client id (must be unique on the broker)
	Username string   `json:"username"`
	Password string   `json:"password"`
	Topics   []string `json:"topics"` // subscriptions, e.g. meshcore/+/+/packets
}

// Default returns a Config populated with sensible defaults for local
// development against the dev MeshCore broker.
func Default() Config {
	return Config{
		ListenAddr:     ":8080",
		DBPath:         "ridgeline.db",
		WebDir:         "web/build",
		ScrubArtifacts: true,
		MQTT: MQTT{
			Broker:   "tcp://localhost:1883",
			ClientID: "ridgelined",
			Topics:   []string{"meshcore/+/+/packets"},
		},
	}
}

// Load reads and parses the config file at path, applying defaults for any
// fields left unset.
func Load(path string) (Config, error) {
	cfg := Default()

	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("config: %w", err)
	}
	// Decode over the defaults so omitted fields keep their default values.
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("config: parsing %s: %w", path, err)
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "ridgeline.db"
	}
	if cfg.MQTT.ClientID == "" {
		cfg.MQTT.ClientID = "ridgelined"
	}
	if len(cfg.MQTT.Topics) == 0 {
		cfg.MQTT.Topics = []string{"meshcore/+/+/packets"}
	}
	return cfg, nil
}
