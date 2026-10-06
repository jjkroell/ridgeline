package api

import (
	"encoding/json"
	"net/http"

	"github.com/jjkroell/ridgeline/internal/store"
)

// channelsDiscovered returns the confirmed hashtag channels and the candidate
// counts. Public: a confirmed name is already decrypting on the live feed, so
// the list reveals nothing the /channels reader doesn't.
func (s *Server) channelsDiscovered(w http.ResponseWriter, _ *http.Request) {
	channels, err := s.store.ConfirmedChannels()
	if err != nil {
		s.fail(w, err)
		return
	}
	pending, confirmed, err := s.store.CountChannelCandidates()
	if err != nil {
		s.fail(w, err)
		return
	}
	if channels == nil {
		channels = []store.ChannelCandidate{}
	}
	writeJSON(w, map[string]any{
		"channels":  channels,
		"pending":   pending,
		"confirmed": confirmed,
	})
}

// channelCandidateCreate records a submitted hashtag name to try and kicks an
// immediate discovery pass. The name is only a candidate — it is listed only if
// it later decrypts real traffic, so submitting a wrong name reveals nothing and
// confirms nothing. Open to anonymous callers (adding a channel to one's browser
// should feed the shared pool whether signed in or not) but IP-rate-limited.
func (s *Server) channelCandidateCreate(w http.ResponseWriter, r *http.Request) {
	if !s.channelLimiter.Allow(clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, "slow down")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request body")
		return
	}
	name, err := s.store.AddChannelCandidate(req.Name, store.ChannelSourceUser, nil)
	if err != nil {
		s.fail(w, err)
		return
	}
	if name == "" {
		writeErr(w, http.StatusBadRequest, "not a usable hashtag channel name (max 31 bytes, no spaces)")
		return
	}
	// Try it now rather than at the next scheduled pass.
	if s.OnChannelCandidate != nil {
		s.OnChannelCandidate()
	}
	writeJSON(w, map[string]any{"ok": true, "name": name})
}
