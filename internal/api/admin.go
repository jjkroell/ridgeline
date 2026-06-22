package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/analytics"
)

// requireAdmin wraps a handler with bearer-token auth against the configured
// admin token. If no admin token is configured the admin API is disabled.
func (s *Server) requireAdmin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.adminToken == "" {
			writeErr(w, http.StatusServiceUnavailable, "admin API disabled (no admin token configured)")
			return
		}
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		// Constant-time compare to avoid leaking the token via timing.
		if subtle.ConstantTimeCompare([]byte(tok), []byte(s.adminToken)) != 1 {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		h(w, r)
	}
}

// adminCheck validates the token (used by the panel to gate its UI).
func (s *Server) adminCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]bool{"ok": true})
}

// adminDetect runs injection detection over the window.
func (s *Server) adminDetect(w http.ResponseWriter, r *http.Request) {
	sinceSec := queryInt(r, "since", 24*3600, 1, 7*86400)
	cutoff := time.Now().Add(-time.Duration(sinceSec) * time.Second).UTC().Format(time.RFC3339Nano)
	nodes, err := s.store.ListNodes()
	if err != nil {
		s.fail(w, err)
		return
	}
	report, err := analytics.DetectInjection(s.store, nodes, cutoff, 0)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, report)
}

func (s *Server) adminBlocklist(w http.ResponseWriter, _ *http.Request) {
	list, err := s.store.ListBlocks()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, list)
}

// blockReq is the body for POST /api/admin/block (quarantine, reversible).
type blockReq struct {
	Kind   string `json:"kind"` // observer | bridge | node
	Key    string `json:"key"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

func (s *Server) adminBlock(w http.ResponseWriter, r *http.Request) {
	var req blockReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request body")
		return
	}
	if !validKind(req.Kind) || req.Key == "" {
		writeErr(w, http.StatusBadRequest, "kind must be observer|bridge|node and key required")
		return
	}
	if err := s.store.AddBlock(req.Kind, req.Key, req.Name, req.Reason); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("admin quarantined", "kind", req.Kind, "key", req.Key, "reason", req.Reason)
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) adminUnblock(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	key := r.URL.Query().Get("key")
	if !validKind(kind) || key == "" {
		writeErr(w, http.StatusBadRequest, "kind and key required")
		return
	}
	if err := s.store.RemoveBlock(kind, key); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("admin un-quarantined", "kind", kind, "key", key)
	writeJSON(w, map[string]bool{"ok": true})
}

// purgeReq is the body for POST /api/admin/purge (hard delete). Each list holds
// the targets to remove; the affected entries are also added to the blocklist so
// purged data does not re-ingest.
type purgeReq struct {
	Observers []string `json:"observers"`
	Bridges   []string `json:"bridges"`
	Nodes     []string `json:"nodes"`
}

func (s *Server) adminPurge(w http.ResponseWriter, r *http.Request) {
	var req purgeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request body")
		return
	}
	if len(req.Observers)+len(req.Bridges)+len(req.Nodes) == 0 {
		writeErr(w, http.StatusBadRequest, "nothing to purge")
		return
	}
	// Block first so nothing re-ingests between the scan and the next packet.
	for _, o := range req.Observers {
		s.store.AddBlock("observer", o, o, "purged")
	}
	for _, b := range req.Bridges {
		s.store.AddBlock("bridge", b, "", "purged")
	}
	for _, n := range req.Nodes {
		s.store.AddBlock("node", n, "", "purged")
	}
	res, err := s.store.PurgeTargets(req.Observers, req.Bridges, req.Nodes)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("admin purged", "observers", len(req.Observers), "bridges", len(req.Bridges),
		"nodes", len(req.Nodes), "observationsDeleted", res.Observations, "nodesDeleted", res.Nodes)
	writeJSON(w, res)
}

func validKind(k string) bool {
	return k == "observer" || k == "bridge" || k == "node"
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
