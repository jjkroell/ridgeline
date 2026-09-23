package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jjkroell/ridgeline/internal/store"
)

// hashSizeSet pins a node's path-hash length to an owner-chosen value, marking it
// manual so the consensus vote (runHashSizeConsensus) stops overriding it. The
// consensus is deliberately slow — it needs a majority of independent flood
// transmissions over a 7-day window — so a node whose owner has just changed its
// hash width would otherwise show the old value for days. Owner-only.
func (s *Server) hashSizeSet(w http.ResponseWriter, r *http.Request, user store.User) {
	pubkey := strings.ToUpper(r.PathValue("pubkey"))
	if !validPubkey(pubkey) {
		writeErr(w, http.StatusBadRequest, "invalid node public key")
		return
	}
	owns, err := s.ownsNode(pubkey, user.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !owns {
		writeErr(w, http.StatusForbidden, "only the node's owner can set its hash-ID length")
		return
	}
	var req struct {
		Size int `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request body")
		return
	}
	if req.Size < 1 || req.Size > 3 {
		writeErr(w, http.StatusBadRequest, "hash-ID length must be 1, 2, or 3 bytes")
		return
	}
	if err := s.store.SetHashSizeManual(pubkey, req.Size); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("hash size pinned by owner", "node", pubkey, "size", req.Size, "user", user.ID)
	writeJSON(w, map[string]any{"hashSize": req.Size, "hashSizeSource": "manual"})
}

// hashSizeClear returns a node to auto-detection. The stored value is kept until
// the next consensus pass produces a confident verdict. Owner-only.
func (s *Server) hashSizeClear(w http.ResponseWriter, r *http.Request, user store.User) {
	pubkey := strings.ToUpper(r.PathValue("pubkey"))
	if !validPubkey(pubkey) {
		writeErr(w, http.StatusBadRequest, "invalid node public key")
		return
	}
	owns, err := s.ownsNode(pubkey, user.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !owns {
		writeErr(w, http.StatusForbidden, "only the node's owner can change its hash-ID length")
		return
	}
	if err := s.store.ClearHashSizeManual(pubkey); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("hash size returned to auto by owner", "node", pubkey, "user", user.ID)
	writeJSON(w, map[string]any{"hashSizeSource": "auto"})
}
