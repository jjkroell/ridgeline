package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

// The build agent's side of the firmware service.
//
// ridgelined never compiles anything. It owns the queue and the database; a
// separate agent process — the only thing here with a docker socket — asks for
// work and reports back. That split is the whole point: the internet-facing
// service cannot be talked into running a compiler, because it has no way to.
//
// These endpoints are for the agent alone and must be blocked at the edge, the
// same way /api/mqtt-auth/* is (see deploy/Caddyfile). The bearer token is the
// second line, not the first.

// FirmwareConfig configures the build service. Disabled (and every endpoint
// 404s) until AgentToken is set, so a deployment that has not deliberately
// turned this on exposes nothing at all.
type FirmwareConfig struct {
	// AgentToken authenticates the build agent.
	AgentToken string
	// ArtifactDir is where finished builds land, as ridgelined sees it.
	ArtifactDir string
	// SourceDir is a MeshCore checkout read only to enumerate the catalogue.
	SourceDir string
	// ArtifactTTLHours is how long a finished build is kept. 0 means seven days.
	ArtifactTTLHours int
	// MaxQueued caps jobs waiting at once. 0 means twenty.
	MaxQueued int
	// Tags are the release tags offered, newest first.
	Tags []string
}

// Enabled reports whether the build service is configured.
func (c FirmwareConfig) Enabled() bool { return c.AgentToken != "" }

// SetFirmware installs the build service configuration.
func (s *Server) SetFirmware(cfg FirmwareConfig) { s.firmware = cfg }

// agentAuthed reports whether the request carries the configured agent token.
func (s *Server) agentAuthed(r *http.Request) bool {
	want := s.firmware.AgentToken
	if want == "" {
		return false // feature disabled; nothing authenticates
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// requireAgent guards the agent endpoints.
//
// An unauthenticated caller gets 404, not 401: these paths should be invisible
// from outside, and a 401 confirms the endpoint exists to anyone probing.
func (s *Server) requireAgent(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.agentAuthed(r) {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}
}

type agentJob struct {
	ID    int64    `json:"id"`
	Tag   string   `json:"tag"`
	Env   string   `json:"env"`
	Flags []string `json:"flags"`
	// OutDir is where the agent must leave artifacts, as the AGENT sees it. It is
	// always <artifactDir>/<job id>; the agent does not get to choose, so a
	// compromised or buggy agent cannot be told to write elsewhere.
	OutDir string `json:"outDir"`
}

// firmwareAgentClaim hands the agent the next queued job, or 204 when idle.
func (s *Server) firmwareAgentClaim(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC().Format(time.RFC3339)
	job, err := s.store.ClaimNextFirmwareJob(now)
	if err != nil {
		s.log.Error("firmware: claim", "err", err)
		http.Error(w, "claim failed", http.StatusInternalServerError)
		return
	}
	if job == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	out := agentJob{
		ID:     job.ID,
		Tag:    job.Tag,
		Env:    job.Env,
		Flags:  strings.Fields(job.Flags),
		OutDir: filepath.Join(s.firmware.ArtifactDir, strconv.FormatInt(job.ID, 10)),
	}
	writeJSON(w, out)
}

type agentResult struct {
	ID    int64  `json:"id"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// firmwareAgentResult records the outcome of a build.
func (s *Server) firmwareAgentResult(w http.ResponseWriter, r *http.Request) {
	var res agentResult
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&res); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	job, err := s.store.GetFirmwareJob(res.ID)
	if err != nil || job == nil {
		http.Error(w, "no such job", http.StatusNotFound)
		return
	}
	// Only a job this agent actually claimed may be completed. Without this a
	// replayed or stray result could mark an unrelated build done and publish an
	// artifact directory that was never written.
	if job.State != store.FirmwareBuilding {
		http.Error(w, "job is not building", http.StatusConflict)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if res.OK {
		dir := filepath.Join(s.firmware.ArtifactDir, strconv.FormatInt(job.ID, 10))
		ttl := time.Duration(s.firmware.ArtifactTTLHours) * time.Hour
		if ttl <= 0 {
			ttl = 7 * 24 * time.Hour
		}
		if err := s.store.FinishFirmwareJob(job.ID, dir, now, ttl); err != nil {
			s.log.Error("firmware: finish", "job", job.ID, "err", err)
			http.Error(w, "could not record completion", http.StatusInternalServerError)
			return
		}
		s.log.Info("firmware: build finished", "job", job.ID, "env", job.Env)
	} else {
		// Keep the END of the message: a build log's last lines carry the error,
		// and the first are always the same setup noise.
		msg := res.Error
		if len(msg) > 300 {
			msg = "…" + msg[len(msg)-300:]
		}
		if err := s.store.FailFirmwareJob(job.ID, msg, now); err != nil {
			s.log.Error("firmware: fail", "job", job.ID, "err", err)
			http.Error(w, "could not record failure", http.StatusInternalServerError)
			return
		}
		s.log.Warn("firmware: build failed", "job", job.ID, "env", job.Env, "err", msg)
	}
	w.WriteHeader(http.StatusNoContent)
}
