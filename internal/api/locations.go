package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jjkroell/ridgeline/internal/store"
)

// maxLocationLabelLen bounds the optional owner label on a private location.
const maxLocationLabelLen = 120

// ownsNode reports whether the user is the node's verified owner. The private
// exact location is strictly owner-only (Phase E will extend read access to
// users the owner explicitly shares with).
func (s *Server) ownsNode(pubkey string, userID int64) (bool, error) {
	owner, ok, err := s.store.NodeOwner(pubkey)
	if err != nil {
		return false, err
	}
	return ok && owner.UserID == userID, nil
}

// privateLocationGet returns the caller's node's private exact location. Only
// the verified owner may read it; everyone else (including logged-in non-owners)
// gets 403 so the endpoint never confirms whether a location exists.
func (s *Server) privateLocationGet(w http.ResponseWriter, r *http.Request, user store.User) {
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
		writeErr(w, http.StatusForbidden, "only the node's owner can view its private location")
		return
	}
	loc, ok, err := s.store.GetPrivateLocation(pubkey)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !ok {
		// No location set yet — return an explicit empty result (not 404) so the
		// owner's editor can distinguish "not set" from "not allowed".
		writeJSON(w, map[string]any{"set": false})
		return
	}
	writeJSON(w, map[string]any{"set": true, "location": loc})
}

// privateLocationSet stores (or replaces) the caller's node's private exact
// location. Owner-only. Coordinates are validated to real lat/lon ranges.
func (s *Server) privateLocationSet(w http.ResponseWriter, r *http.Request, user store.User) {
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
		writeErr(w, http.StatusForbidden, "only the node's owner can set its private location")
		return
	}
	var req struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Label     string  `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request body")
		return
	}
	if req.Latitude < -90 || req.Latitude > 90 || req.Longitude < -180 || req.Longitude > 180 {
		writeErr(w, http.StatusBadRequest, "latitude must be -90..90 and longitude -180..180")
		return
	}
	label := strings.TrimSpace(req.Label)
	if len(label) > maxLocationLabelLen {
		writeErr(w, http.StatusBadRequest, "label is too long")
		return
	}
	loc, err := s.store.SetPrivateLocation(pubkey, user.ID, req.Latitude, req.Longitude, label)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("private location set", "node", pubkey, "user", user.ID)
	writeJSON(w, map[string]any{"set": true, "location": loc})
}

// privateLocationDelete clears the caller's node's private exact location.
func (s *Server) privateLocationDelete(w http.ResponseWriter, r *http.Request, user store.User) {
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
		writeErr(w, http.StatusForbidden, "only the node's owner can clear its private location")
		return
	}
	if _, err := s.store.DeletePrivateLocation(pubkey); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("private location cleared", "node", pubkey, "user", user.ID)
	writeJSON(w, map[string]bool{"ok": true})
}
