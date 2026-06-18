// Package api serves ridgelined's REST endpoints and the live WebSocket feed
// over the store.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jjkroell/ridgeline/internal/analytics"
	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// Server holds API dependencies and serves HTTP.
type Server struct {
	store     *store.Store
	log       *slog.Logger
	version   string
	webDir    string
	hub       *hub
	up        websocket.Upgrader
	analytics *analytics.Engine
}

// SetAnalytics attaches the analytics engine used by the node-detail endpoint.
func (s *Server) SetAnalytics(e *analytics.Engine) { s.analytics = e }

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
	mux.HandleFunc("GET /api/nodes/{pubkey}", s.nodeDetail)
	mux.HandleFunc("GET /api/observers", s.observers)
	mux.HandleFunc("GET /api/observations", s.observations)
	mux.HandleFunc("GET /api/recent", s.recent)
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
	// GroupText channel fields. ChannelHash is always set for GroupText; the
	// rest are populated only when the message decrypts (e.g. public channel).
	ChannelHash string `json:"channelHash,omitempty"`
	Channel     string `json:"channel,omitempty"`
	Sender      string `json:"sender,omitempty"`
	Text        string `json:"text,omitempty"`
	ObserverID  string `json:"observerId,omitempty"`
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

// newLiveEvent builds the JSON event shape from a decoded packet and its
// reception envelope. Shared by the live broadcast and the /api/recent replay
// so both render identically.
func newLiveEvent(pkt *meshcore.Packet, rawHex, observerID, region, receivedAt string, snr, rssi *float64) LiveEvent {
	ev := LiveEvent{
		MessageHash:    pkt.MessageHash,
		RouteType:      pkt.RouteType.String(),
		PayloadType:    pkt.PayloadType.String(),
		PayloadVersion: pkt.PayloadVersion,
		PathHops:       pkt.PathHopCount,
		HashSize:       pkt.PathHashSize,
		Path:           pkt.Path,
		TransportCodes: pkt.TransportCodes,
		PayloadRaw:     pkt.PayloadRaw,
		Raw:            strings.ToUpper(rawHex),
		ObserverID:     observerID,
		Region:         region,
		SNR:            snr,
		RSSI:           rssi,
		ReceivedAt:     receivedAt,
	}
	if a := pkt.Advert; a != nil {
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
	if gt := pkt.GroupText; gt != nil {
		ev.ChannelHash = gt.ChannelHash
		if gt.Decrypted {
			ev.Channel = gt.Channel
			ev.Sender = gt.Sender
			ev.Text = gt.Message
		}
	}
	return ev
}

// Broadcast pushes an observation to live WebSocket subscribers.
func (s *Server) Broadcast(o store.Observation) {
	ev := newLiveEvent(o.Packet, o.RawHex, o.ObserverID, o.Region,
		o.ReceivedAt.UTC().Format(time.RFC3339Nano), o.SNR, o.RSSI)
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

// nodeDetail returns one node's row plus its computed analytics snapshot.
func (s *Server) nodeDetail(w http.ResponseWriter, r *http.Request) {
	pubkey := strings.ToUpper(r.PathValue("pubkey"))
	nodes, err := s.store.ListNodes()
	if err != nil {
		s.fail(w, err)
		return
	}
	var node *store.Node
	for i := range nodes {
		if strings.ToUpper(nodes[i].PublicKey) == pubkey {
			node = &nodes[i]
			break
		}
	}
	resp := struct {
		Node        *store.Node           `json:"node"`
		Detail      *analytics.NodeDetail `json:"detail"`
		GeneratedAt string                `json:"generatedAt,omitempty"`
	}{Node: node}
	if s.analytics != nil && node != nil {
		d, gen := s.analytics.Get(node.PublicKey)
		resp.Detail = d
		if !gen.IsZero() {
			resp.GeneratedAt = gen.UTC().Format(time.RFC3339)
		}
	}
	writeJSON(w, resp)
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

// recent returns the last `since` seconds (default 1h, max 6h) of observations
// re-decoded into the same shape as live WebSocket events, newest first, so the
// feed can render history identically without waiting for fresh packets.
func (s *Server) recent(w http.ResponseWriter, r *http.Request) {
	sinceSec := 3600
	if v := r.URL.Query().Get("since"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sinceSec = n
		}
	}
	if sinceSec > 6*3600 {
		sinceSec = 6 * 3600
	}
	cutoff := time.Now().Add(-time.Duration(sinceSec) * time.Second).UTC().Format(time.RFC3339Nano)

	raws, err := s.store.RecentRaw(cutoff, 3000)
	if err != nil {
		s.fail(w, err)
		return
	}

	out := make([]LiveEvent, 0, len(raws))
	for _, ro := range raws {
		pkt, err := meshcore.DecodeHex(ro.RawHex)
		if err != nil || pkt == nil {
			continue
		}
		out = append(out, newLiveEvent(pkt, ro.RawHex, ro.ObserverID, ro.Region, ro.ReceivedAt, ro.SNR, ro.RSSI))
	}
	writeJSON(w, out)
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
