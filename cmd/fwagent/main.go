// Command fwagent builds MeshCore firmware on behalf of ridgelined.
//
// It exists so that ridgelined does not have to. Compiling needs a docker
// daemon, and a docker socket is root on the host — handing one to the
// internet-facing service would mean a bug there becomes a host compromise.
// Instead this agent holds the socket, reaches ridgelined over the internal
// network, and never listens on anything itself. It asks for work; it is never
// asked to do any.
//
// Usage:
//
//	fwagent -api http://ridgelined:8080 -token <agent token>
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type job struct {
	ID     int64    `json:"id"`
	Tag    string   `json:"tag"`
	Env    string   `json:"env"`
	Flags  []string `json:"flags"`
	OutDir string   `json:"outDir"`
}

type result struct {
	ID    int64  `json:"id"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func main() {
	var (
		apiURL    = flag.String("api", "http://ridgelined:8080", "ridgelined base URL")
		token     = flag.String("token", os.Getenv("FWAGENT_TOKEN"), "agent token (or FWAGENT_TOKEN)")
		image     = flag.String("image", "meshcore-builder:latest", "builder image")
		pioVol    = flag.String("pio-volume", "meshcore-pio", "docker volume for PlatformIO toolchains")
		srcVol    = flag.String("src-volume", "meshcore-src", "docker volume for the MeshCore checkout")
		outVol    = flag.String("out-volume", "ridgeline-firmware", "docker volume artifacts are written to")
		cpus      = flag.String("cpus", "2", "CPU limit for a build container")
		poll      = flag.Duration("poll", 5*time.Second, "how often to ask for work")
		buildTime = flag.Duration("build-timeout", 30*time.Minute, "hard cap on one build")
	)
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if *token == "" {
		log.Error("fwagent: -token is required")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a := &agent{
		api: strings.TrimRight(*apiURL, "/"), token: *token, image: *image,
		pioVol: *pioVol, srcVol: *srcVol, outVol: *outVol, cpus: *cpus,
		buildTimeout: *buildTime, log: log,
		// A short client timeout would abandon a claim mid-flight and leave the job
		// stuck building; these calls are tiny, so this only guards a hung server.
		http: &http.Client{Timeout: 30 * time.Second},
	}
	log.Info("fwagent: started", "api", a.api, "image", a.image)

	t := time.NewTicker(*poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("fwagent: stopping")
			return
		case <-t.C:
			// Drain the queue rather than taking one job per tick, so a backlog is
			// not paced by the poll interval.
			for a.once(ctx) {
				if ctx.Err() != nil {
					return
				}
			}
		}
	}
}

type agent struct {
	api, token, image      string
	pioVol, srcVol, outVol string
	cpus                   string
	buildTimeout           time.Duration
	http                   *http.Client
	log                    *slog.Logger
}

// once claims and runs at most one job, reporting whether it found work.
func (a *agent) once(ctx context.Context) bool {
	j, err := a.claim(ctx)
	if err != nil {
		a.log.Error("fwagent: claim", "err", err)
		return false
	}
	if j == nil {
		return false
	}
	log := a.log.With("job", j.ID, "env", j.Env, "tag", j.Tag)
	log.Info("fwagent: building", "flags", strings.Join(j.Flags, " "))

	start := time.Now()
	buildCtx, cancel := context.WithTimeout(ctx, a.buildTimeout)
	out, err := a.build(buildCtx, j)
	cancel()

	if err != nil {
		log.Error("fwagent: build failed", "err", err, "took", time.Since(start).Round(time.Second))
		// The requester gets the tail of the compiler output, which is where the
		// actual error is; the full log stays here.
		a.report(ctx, result{ID: j.ID, OK: false, Error: tail(out, 10)})
		return true
	}
	log.Info("fwagent: built", "took", time.Since(start).Round(time.Second))
	a.report(ctx, result{ID: j.ID, OK: true})
	return true
}

func (a *agent) claim(ctx context.Context) (*job, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.api+"/api/firmware/agent/claim", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil, nil // idle
	case http.StatusOK:
		var j job
		if err := json.NewDecoder(resp.Body).Decode(&j); err != nil {
			return nil, err
		}
		return &j, nil
	case http.StatusNotFound:
		// The endpoints 404 when the token is wrong or the feature is off. Say so
		// plainly rather than looking like an idle queue forever.
		return nil, fmt.Errorf("claim refused (404): wrong token, or the build service is disabled")
	default:
		return nil, fmt.Errorf("claim: unexpected status %s", resp.Status)
	}
}

func (a *agent) report(ctx context.Context, res result) {
	body, _ := json.Marshal(res)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.api+"/api/firmware/agent/result", bytes.NewReader(body))
	if err != nil {
		a.log.Error("fwagent: build result request", "err", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.http.Do(req)
	if err != nil {
		a.log.Error("fwagent: report result", "job", res.ID, "err", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		a.log.Error("fwagent: result rejected", "job", res.ID, "status", resp.Status)
	}
}

// build launches the builder image as a SIBLING container.
//
// Volume arguments are docker NAMES, never paths: the daemon resolves them on
// the host, and a path that exists inside this container would mean nothing
// there. The job's OutDir is a path under the shared artifact volume, so the
// container is given the volume and told which subdirectory to write.
func (a *agent) build(ctx context.Context, j *job) (string, error) {
	args := []string{
		"run", "--rm",
		"--cpus=" + a.cpus,
		"-v", a.pioVol + ":/pio",
		"-v", a.srcVol + ":/src",
		"-v", a.outVol + ":/artifacts",
		// build.sh writes to OUT_DIR; ridgelined dictates the path so a build can
		// never be steered at a directory of the agent's choosing.
		"-e", "OUT_DIR=" + containerOut(j),
		a.image, j.Tag, j.Env,
	}
	args = append(args, j.Flags...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// containerOut maps the job's artifact directory into the builder's mount.
//
// ridgelined names the directory by job id under its own artifact root; the
// builder sees the same volume at /artifacts, so only the last element travels.
func containerOut(j *job) string {
	return "/artifacts/" + strconv.FormatInt(j.ID, 10)
}

// tail keeps the last n lines of a build log, then hard-caps the result.
//
// The line cap alone is not enough: PlatformIO answers an unknown environment by
// listing every valid name, which is one line of several hundred. Sending that
// unbounded overran the server's request limit, the result was rejected, and the
// job sat in "building" forever — so the byte cap is what actually protects the
// report, and the line cap is only there to keep it readable.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := strings.Join(lines, "\n")
	// Trim from the FRONT, not the back. A compiler failure puts the reason last,
	// so cutting the tail leaves the requester reading setup chatter and none of
	// the error — which is what happened the first time this was tested.
	const maxBytes = 1500
	if len(out) > maxBytes {
		out = "…" + out[len(out)-maxBytes:]
	}
	return out
}
