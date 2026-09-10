package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/firmware"
	"github.com/jjkroell/ridgeline/internal/store"
)

// The requester's side of the firmware build service. The agent's side lives in
// firmware_agent.go; nothing here compiles anything or talks to docker.

// artifactNames is the complete set of files a build may publish.
//
// An allowlist, deliberately, not a path sanitiser. The download path takes a
// filename from the URL, and "clean it and hope" is how directory traversal bugs
// happen; a fixed set of names cannot traverse anywhere.
var artifactNames = map[string]bool{
	"firmware-merged.bin": true, // ESP32: the single flashable image
	"firmware.zip":        true, // nRF52: serial DFU package
	"firmware.uf2":        true, // nRF52: drag-and-drop
	"firmware.bin":        true,
	"firmware.hex":        true,
	"bootloader.bin":      true, // ESP32 components, for a bench esptool flash
	"partitions.bin":      true,
}

type firmwareCatalogue struct {
	Tags    []string          `json:"tags"`
	Boards  []firmware.Board  `json:"boards"`
	Options []firmware.Option `json:"options"`
}

// firmwareCatalogue lists what can be built: vetted release tags, the boards and
// environments read from the source tree, and the option allowlist.
func (s *Server) firmwareCatalogue(w http.ResponseWriter, r *http.Request, _ store.User) {
	if !s.firmware.Enabled() {
		writeErr(w, http.StatusNotFound, "firmware builds are not enabled")
		return
	}
	boards, err := firmware.LoadCatalogue(s.firmware.SourceDir)
	if err != nil {
		s.log.Error("firmware: load catalogue", "err", err)
		writeErr(w, http.StatusInternalServerError, "could not read the firmware catalogue")
		return
	}
	writeJSON(w, firmwareCatalogue{
		Tags:    s.firmware.Tags,
		Boards:  boards,
		Options: firmware.Options,
	})
}

type firmwareBuildReq struct {
	Tag     string   `json:"tag"`
	Env     string   `json:"env"`
	Options []string `json:"options"`
}

type firmwareBuildResp struct {
	Job *store.FirmwareJob `json:"job"`
	// Cached reports that an identical build already existed, so nothing was
	// queued. The UI uses it to skip straight to the download.
	Cached bool `json:"cached"`
}

// firmwareBuild queues a build, or hands back an equivalent one already done or
// in flight.
func (s *Server) firmwareBuild(w http.ResponseWriter, r *http.Request, u store.User) {
	if !s.firmware.Enabled() {
		writeErr(w, http.StatusNotFound, "firmware builds are not enabled")
		return
	}
	var req firmwareBuildReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "could not read the request")
		return
	}

	// The tag must be one the operator vetted. Without this the git tag is
	// attacker-chosen text that the agent hands to `git fetch`.
	if !contains(s.firmware.Tags, req.Tag) {
		writeErr(w, http.StatusBadRequest, "that release is not available to build")
		return
	}
	boards, err := firmware.LoadCatalogue(s.firmware.SourceDir)
	if err != nil {
		s.log.Error("firmware: load catalogue", "err", err)
		writeErr(w, http.StatusInternalServerError, "could not read the firmware catalogue")
		return
	}
	// ResolveFlags is the security boundary: it maps option IDS to flags we wrote,
	// and refuses anything not in the allowlist for this environment. No text from
	// the request ever becomes a compiler flag.
	flags, err := firmware.ResolveFlags(boards, req.Env, req.Options)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	canon := store.CanonicalFlags(flags)
	key := store.FirmwareCacheKey(req.Tag, req.Env, canon)
	now := time.Now().UTC().Format(time.RFC3339)

	// Identical firmware is never compiled twice: a finished build whose artifacts
	// are still on disk is handed straight back.
	if done, err := s.store.FindUsableFirmware(key, now); err != nil {
		s.log.Error("firmware: cache lookup", "err", err)
	} else if done != nil {
		writeJSON(w, firmwareBuildResp{Job: done, Cached: true})
		return
	}
	// And a request for something already building joins that job rather than
	// queueing a second identical compile.
	if active, err := s.store.FindActiveFirmware(key); err != nil {
		s.log.Error("firmware: active lookup", "err", err)
	} else if active != nil {
		writeJSON(w, firmwareBuildResp{Job: active})
		return
	}

	// Rate limit per account, not per IP: builds are expensive and the endpoint
	// already requires a session, so the account is the meaningful identity.
	if !s.firmwareLimiter.Allow(strconv.FormatInt(u.ID, 10)) {
		writeErr(w, http.StatusTooManyRequests, "too many builds requested, please wait a little")
		return
	}
	max := s.firmware.MaxQueued
	if max <= 0 {
		max = 20
	}
	if n, err := s.store.CountQueuedFirmware(); err != nil {
		s.log.Error("firmware: count queued", "err", err)
	} else if n >= int64(max) {
		writeErr(w, http.StatusServiceUnavailable, "the build queue is full, please try again shortly")
		return
	}

	id, err := s.store.EnqueueFirmware(key, req.Tag, req.Env, canon, u.ID, now)
	if err != nil {
		s.log.Error("firmware: enqueue", "err", err)
		writeErr(w, http.StatusInternalServerError, "could not queue the build")
		return
	}
	job, err := s.store.GetFirmwareJob(id)
	if err != nil || job == nil {
		writeErr(w, http.StatusInternalServerError, "could not read the queued build")
		return
	}
	s.log.Info("firmware: queued", "job", id, "env", req.Env, "tag", req.Tag, "user", u.ID)
	writeJSON(w, firmwareBuildResp{Job: job})
}

type firmwareJobResp struct {
	Job       *store.FirmwareJob `json:"job"`
	Artifacts []firmwareArtifact `json:"artifacts"`
	// Queued is how many jobs sit ahead of this one, so the UI can say something
	// truer than "please wait" while a build is pending.
	Ahead int64 `json:"ahead"`
}

type firmwareArtifact struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
	// DownloadName is what the file is called once saved. Surfaced so the list
	// shows the same name that lands on disk.
	DownloadName string `json:"downloadName"`
}

// downloadName builds a filename that says what the firmware IS.
//
// The build directory names everything firmware.*, which is fine on the server
// and useless in a downloads folder: two builds of the same board differing only
// by option would arrive identically named. The tag is reduced to its version
// too — MeshCore tags each firmware line separately, so a companion build
// carries the tag "repeater-v1.17.1", which reads as a mistake.
//
// Result: t1000e_companion_radio_ble-1.17.1-buzzer-quiet.uf2
func downloadName(env, tag, flags, artifact string) string {
	ext := filepath.Ext(artifact)
	base := strings.TrimSuffix(artifact, ext)

	parts := []string{safeName(env), releaseVersion(tag)}
	// The ESP32 component images are only meaningful next to the merged file, so
	// they keep their role in the name and skip the option suffix: they are
	// identical across option sets anyway.
	switch base {
	case "bootloader", "partitions":
		parts = append(parts, base)
		return strings.Join(parts, "-") + ext
	case "firmware-merged":
		parts = append(parts, "merged")
	}
	if opts := firmware.OptionIDsForFlags(flags); len(opts) > 0 {
		// "+" between options: readable, filesystem-safe, and unambiguous where the
		// option ids themselves contain hyphens.
		parts = append(parts, strings.Join(opts, "+"))
	}
	return strings.Join(parts, "-") + ext
}

var semver = regexp.MustCompile(`\d+\.\d+\.\d+(?:\.\d+)?`)

// releaseVersion reduces "repeater-v1.17.1" to "1.17.1", falling back to the
// whole tag when it carries no version.
func releaseVersion(tag string) string {
	if m := semver.FindString(tag); m != "" {
		return m
	}
	return safeName(tag)
}

// safeName keeps a filename to characters that cannot upset a Content-Disposition
// header or a filesystem. Values here come from our own catalogue, so this is a
// belt-and-braces measure rather than a load-bearing one.
func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '_', r == '-', r == '+':
			return r
		}
		return '_'
	}, s)
}

// firmwareJob reports a build's progress and, once done, what it produced.
func (s *Server) firmwareJob(w http.ResponseWriter, r *http.Request, _ store.User) {
	job, ok := s.lookupFirmwareJob(w, r)
	if !ok {
		return
	}
	resp := firmwareJobResp{Job: job}
	if job.State == store.FirmwareDone && job.ArtifactDir != "" {
		resp.Artifacts = listArtifacts(job)
	}
	if job.State == store.FirmwareQueued {
		if n, err := s.store.CountFirmwareAhead(job.ID); err == nil {
			resp.Ahead = n
		}
	}
	writeJSON(w, resp)
}

// firmwareDownload serves one artifact of a finished build.
func (s *Server) firmwareDownload(w http.ResponseWriter, r *http.Request, _ store.User) {
	job, ok := s.lookupFirmwareJob(w, r)
	if !ok {
		return
	}
	if job.State != store.FirmwareDone || job.ArtifactDir == "" {
		writeErr(w, http.StatusNotFound, "that build has no artifacts")
		return
	}
	name := r.PathValue("name")
	if !artifactNames[name] {
		writeErr(w, http.StatusNotFound, "no such artifact")
		return
	}
	path := filepath.Join(job.ArtifactDir, name)
	f, err := os.Open(path)
	if err != nil {
		// Expired and swept between the status call and this one is the common case.
		writeErr(w, http.StatusNotFound, "that artifact is no longer available")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		writeErr(w, http.StatusNotFound, "that artifact is no longer available")
		return
	}
	filename := downloadName(job.Env, job.Tag, job.Flags, name)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	http.ServeContent(w, r, name, st.ModTime(), f)
}

type firmwareBuildSummary struct {
	ID         int64              `json:"id"`
	Tag        string             `json:"tag"`
	Env        string             `json:"env"`
	Options    []string           `json:"options"`
	FinishedAt *string            `json:"finishedAt,omitempty"`
	ExpiresAt  *string            `json:"expiresAt,omitempty"`
	Artifacts  []firmwareArtifact `json:"artifacts"`
}

// firmwareBuilds lists finished builds of one environment that are still
// downloadable, so a combination someone has already compiled can be taken
// rather than built again.
func (s *Server) firmwareBuilds(w http.ResponseWriter, r *http.Request, _ store.User) {
	if !s.firmware.Enabled() {
		writeErr(w, http.StatusNotFound, "firmware builds are not enabled")
		return
	}
	env := r.URL.Query().Get("env")
	if env == "" {
		writeErr(w, http.StatusBadRequest, "env is required")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	jobs, err := s.store.RecentFirmwareBuilds(env, 20, now)
	if err != nil {
		s.log.Error("firmware: recent builds", "err", err)
		writeErr(w, http.StatusInternalServerError, "could not list previous builds")
		return
	}
	out := make([]firmwareBuildSummary, 0, len(jobs))
	for _, j := range jobs {
		arts := listArtifacts(&j)
		// A row whose directory has already been swept is not offerable; skip it
		// rather than show a download that 404s.
		if len(arts) == 0 {
			continue
		}
		out = append(out, firmwareBuildSummary{
			ID:         j.ID,
			Tag:        j.Tag,
			Env:        j.Env,
			Options:    firmware.OptionIDsForFlags(j.Flags),
			FinishedAt: j.FinishedAt,
			ExpiresAt:  j.ExpiresAt,
			Artifacts:  arts,
		})
	}
	writeJSON(w, out)
}

func (s *Server) lookupFirmwareJob(w http.ResponseWriter, r *http.Request) (*store.FirmwareJob, bool) {
	if !s.firmware.Enabled() {
		writeErr(w, http.StatusNotFound, "firmware builds are not enabled")
		return nil, false
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad job id")
		return nil, false
	}
	job, err := s.store.GetFirmwareJob(id)
	if err != nil {
		s.log.Error("firmware: get job", "err", err)
		writeErr(w, http.StatusInternalServerError, "could not read that build")
		return nil, false
	}
	if job == nil {
		writeErr(w, http.StatusNotFound, "no such build")
		return nil, false
	}
	// Deliberately NOT restricted to the requester: a build is a pure function of
	// (release, firmware, options), carries nothing about who asked, and two
	// people choosing the same thing get the same job by design.
	return job, true
}

func listArtifacts(job *store.FirmwareJob) []firmwareArtifact {
	entries, err := os.ReadDir(job.ArtifactDir)
	if err != nil {
		return nil
	}
	var out []firmwareArtifact
	for _, e := range entries {
		if e.IsDir() || !artifactNames[e.Name()] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, firmwareArtifact{
			Name:         e.Name(),
			Bytes:        info.Size(),
			DownloadName: downloadName(job.Env, job.Tag, job.Flags, e.Name()),
		})
	}
	return out
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
