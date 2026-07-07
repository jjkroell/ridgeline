package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/jjkroell/ridgeline/internal/store"
)

// locationSharesList returns the users a node's private location is shared with.
// Owner-only.
func (s *Server) locationSharesList(w http.ResponseWriter, r *http.Request, user store.User) {
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
		writeErr(w, http.StatusForbidden, "only the node's owner can manage location sharing")
		return
	}
	shares, err := s.store.ListLocationShares(pubkey)
	if err != nil {
		s.fail(w, err)
		return
	}
	if shares == nil {
		shares = []store.LocationShare{}
	}
	writeJSON(w, shares)
}

// locationShareCreate grants a registered user (by email) read access to a
// node's private location. Owner-only.
func (s *Server) locationShareCreate(w http.ResponseWriter, r *http.Request, user store.User) {
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
		writeErr(w, http.StatusForbidden, "only the node's owner can share this location")
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		writeErr(w, http.StatusBadRequest, "an email address is required")
		return
	}
	grantee, ok, err := s.store.GetUserByEmail(email)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "no registered user with that email")
		return
	}
	if grantee.ID == user.ID {
		writeErr(w, http.StatusBadRequest, "you already own this node")
		return
	}
	if grantee.Blocked {
		writeErr(w, http.StatusBadRequest, "that account is suspended")
		return
	}
	if err := s.store.ShareLocation(pubkey, user.ID, grantee.ID); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("private location shared", "node", pubkey, "owner", user.ID, "grantee", grantee.ID)
	// Return the refreshed list so the UI can re-render in one round trip.
	shares, err := s.store.ListLocationShares(pubkey)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, shares)
}

// locationShareDelete revokes a grantee's access. Owner-only.
func (s *Server) locationShareDelete(w http.ResponseWriter, r *http.Request, user store.User) {
	pubkey := strings.ToUpper(r.PathValue("pubkey"))
	if !validPubkey(pubkey) {
		writeErr(w, http.StatusBadRequest, "invalid node public key")
		return
	}
	granteeID, err := strconv.ParseInt(r.PathValue("userId"), 10, 64)
	if err != nil || granteeID <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid user id")
		return
	}
	owns, err := s.ownsNode(pubkey, user.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !owns {
		writeErr(w, http.StatusForbidden, "only the node's owner can revoke sharing")
		return
	}
	if _, err := s.store.UnshareLocation(pubkey, granteeID); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("private location share revoked", "node", pubkey, "owner", user.ID, "grantee", granteeID)
	writeJSON(w, map[string]bool{"ok": true})
}
