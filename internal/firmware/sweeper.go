package firmware

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

// Sweeper is ridgelined's only moving part in the build service.
//
// Compiling happens in a separate agent process holding the docker socket (see
// cmd/fwagent); ridgelined owns the queue and the artifacts on disk. What is
// left here is housekeeping: reaping expired builds, and rescuing jobs the
// agent abandoned.
type Sweeper struct {
	st  *store.Store
	log *slog.Logger

	// Interval between sweeps. Artifact expiry is measured in days, so this is
	// deliberately lazy.
	Interval time.Duration

	// StaleAfter is how long a job may sit in "building" before it is presumed
	// abandoned and returned to the queue. Must comfortably exceed the agent's
	// build timeout, or a slow-but-healthy build gets requeued underneath itself
	// and compiled twice.
	StaleAfter time.Duration
}

func NewSweeper(st *store.Store, log *slog.Logger) *Sweeper {
	return &Sweeper{st: st, log: log, Interval: time.Hour, StaleAfter: 90 * time.Minute}
}

// Run sweeps until ctx is cancelled.
func (s *Sweeper) Run(ctx context.Context) {
	// A job left "building" across a restart belongs to an agent that is gone.
	// Nothing will finish it, and every later request for the same firmware would
	// join a build that is not running.
	if n, err := s.st.RequeueStaleFirmwareJobs(); err != nil {
		s.log.Warn("firmware: requeue interrupted jobs", "err", err)
	} else if n > 0 {
		s.log.Info("firmware: requeued jobs interrupted by a restart", "count", n)
	}

	t := time.NewTicker(s.Interval)
	defer t.Stop()
	s.sweep()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.sweep()
			s.rescue()
		}
	}
}

// rescue returns builds abandoned mid-flight to the queue.
//
// An agent can die, or have its result rejected, after claiming a job. Nothing
// then finishes it, and the cache lookup happily joins later requests to a build
// that is not running — so a job older than StaleAfter is presumed abandoned.
// StaleAfter must comfortably exceed the agent's own build timeout, or a slow but
// healthy build gets requeued underneath itself and compiled twice.
func (s *Sweeper) rescue() {
	cutoff := time.Now().UTC().Add(-s.StaleAfter).Format(time.RFC3339)
	n, err := s.st.RequeueAbandonedFirmwareJobs(cutoff)
	if err != nil {
		s.log.Error("firmware: rescue abandoned builds", "err", err)
		return
	}
	if n > 0 {
		s.log.Warn("firmware: requeued abandoned builds", "count", n, "olderThan", s.StaleAfter)
	}
}

// sweep deletes artifacts past their expiry, clearing the pointer but keeping
// the job row — that row is the record that a build happened and who asked for
// it, which outlives the file it produced.
func (s *Sweeper) sweep() {
	now := time.Now().UTC().Format(time.RFC3339)
	jobs, err := s.st.ExpiredFirmwareJobs(now)
	if err != nil {
		s.log.Error("firmware: list expired builds", "err", err)
		return
	}
	for _, j := range jobs {
		if err := os.RemoveAll(j.ArtifactDir); err != nil {
			s.log.Warn("firmware: remove artifacts", "job", j.ID, "err", err)
			continue
		}
		if err := s.st.ForgetFirmwareArtifacts(j.ID); err != nil {
			s.log.Warn("firmware: clear artifact pointer", "job", j.ID, "err", err)
		}
	}
	if len(jobs) > 0 {
		s.log.Info("firmware: swept expired builds", "count", len(jobs))
	}
}
