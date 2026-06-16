// Package api serves ridgelined's REST endpoints and the live WebSocket feed
// over the store.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jjkroell/ridgeline/internal/store"
)

// Server holds API dependencies and serves HTTP.
type Server struct {
	store   *store.Store
	log     *slog.Logger
	version string
	webDir  string
	hub     *hub
	up      websocket.Upgrader
}

// New creates an API Server. If webDir is non-empty and exists, the built SPA
// is served from it with an index.html fallback for client routes.
func New(st *store.Store, log *slog.Logger, version, webDir string) *Server {
	return &Server{
		store:   st,
		log:     log,
		version: version,
		webDir:  webDir,
		hub:     newHub(),
		// Dev: allow any origin. Tighten before exposing publicly.
		up: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

// Handler returns the configured HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/stats", s.stats)
	mux.HandleFunc("GET /api/nodes", s.nodes)
	mux.HandleFunc("GET /api/observers", s.observers)
	mux.HandleFunc("GET /api/observations", s.observations)
	mux.HandleFunc("GET /api/live", s.live)

	if s.webDir != "" {
		if info, err := os.Stat(s.webDir); err == nil && info.IsDir() {
			mux.HandleFunc("/", staticHandler(s.webDir))
			s.log.Info("serving web UI", "dir", s.webDir)
		} else {
			s.log.Warn("web dir not found, serving API only", "dir", s.webDir)
		}
	}
	return mux
}

// LiveEvent is the JSON shape broadcast to WebSocket clients per observation.
type LiveEvent struct {
	MessageHash    string `json:"messageHash"`
	RouteType      string `json:"routeType"`
	PayloadType    string `json:"payloadType"`
	PayloadVersion uint8  `json:"payloadVersion"`
	PathHops       int    `json:"pathHops"`
	HashSize       int    `json:"hashSize"`
	// Path holds the per-hop relay key prefixes (uppercase hex) the packet
	// accumulated as it flooded — the chain of repeaters that relayed it.
	Path           []string  `json:"path,omitempty"`
	TransportCodes *[2]uint16 `json:"transportCodes,omitempty"`
	PayloadRaw     string    `json:"payloadRaw,omitempty"`
	Raw            string    `json:"raw,omitempty"`
	ObserverID     string    `json:"observerId,omitempty"`
	Region         string    `json:"region,omitempty"`
	SNR            *float64  `json:"snr,omitempty"`
	RSSI           *float64  `json:"rssi,omitempty"`
	ReceivedAt     string    `json:"receivedAt"`
	// Node is populated for Advert packets.
	Node *LiveNode `json:"node,omitempty"`
}

// LiveNode summarizes the node announced by an Advert.
type LiveNode struct {
	PublicKey string   `json:"publicKey"`
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Timestamp uint32   `json:"timestamp,omitempty"` // advertised unix time
}

// Broadcast pushes an observation to live WebSocket subscribers.
func (s *Server) Broadcast(o store.Observation) {
	ev := LiveEvent{
		MessageHash:    o.Packet.MessageHash,
		RouteType:      o.Packet.RouteType.String(),
		PayloadType:    o.Packet.PayloadType.String(),
		PayloadVersion: o.Packet.PayloadVersion,
		PathHops:       o.Packet.PathHopCount,
		HashSize:       o.Packet.PathHashSize,
		Path:           o.Packet.Path,
		TransportCodes: o.Packet.TransportCodes,
		PayloadRaw:     o.Packet.PayloadRaw,
		Raw:            o.RawHex,
		ObserverID:     o.ObserverID,
		Region:         o.Region,
		SNR:            o.SNR,
		RSSI:           o.RSSI,
		ReceivedAt:     o.ReceivedAt.UTC().Format(time.RFC3339Nano),
	}
	if a := o.Packet.Advert; a != nil {
		n := &LiveNode{
			PublicKey: a.PublicKey,
			Name:      a.Name,
			Role:      a.DeviceRole.String(),
			Timestamp: a.Timestamp,
		}
		if a.HasLocation {
			lat, lon := a.Latitude, a.Longitude
			n.Latitude, n.Longitude = &lat, &lon
		}
		ev.Node = n
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	s.hub.broadcast(b)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "version": s.version})
}

func (s *Server) stats(w http.ResponseWriter, _ *http.Request) {
	st, err := s.store.Stats()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, st)
}

func (s *Server) nodes(w http.ResponseWriter, _ *http.Request) {
	nodes, err := s.store.ListNodes()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, nodes)
}

func (s *Server) observers(w http.ResponseWriter, _ *http.Request) {
	obs, err := s.store.ListObservers()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, obs)
}

func (s *Server) observations(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	obs, err := s.store.RecentObservations(limit)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, obs)
}

func (s *Server) live(w http.ResponseWriter, r *http.Request) {
	conn, err := s.up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ch := s.hub.add(conn)
	defer s.hub.remove(conn)

	// Reader pump: discard input, detect close.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				conn.Close()
				return
			}
		}
	}()

	for msg := range ch {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	s.log.Error("api error", "err", err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
