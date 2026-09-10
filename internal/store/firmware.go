package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// Firmware build job states.
const (
	FirmwareQueued   = "queued"
	FirmwareBuilding = "building"
	FirmwareDone     = "done"
	FirmwareFailed   = "failed"
)

// FirmwareJob is one firmware build request and its outcome.
type FirmwareJob struct {
	ID          int64   `json:"id"`
	CacheKey    string  `json:"-"`
	Tag         string  `json:"tag"`
	Env         string  `json:"env"`
	Flags       string  `json:"flags"`
	State       string  `json:"state"`
	RequestedBy int64   `json:"-"`
	Error       string  `json:"error,omitempty"`
	ArtifactDir string  `json:"-"`
	CreatedAt   string  `json:"createdAt"`
	StartedAt   *string `json:"startedAt,omitempty"`
	FinishedAt  *string `json:"finishedAt,omitempty"`
	ExpiresAt   *string `json:"expiresAt,omitempty"`
}

// CanonicalFlags normalises an option set so equivalent requests hash alike.
//
// Order carries no meaning to the compiler — "-D A=1 -D B=1" and "-D B=1 -D A=1"
// produce identical firmware — so the cache must not treat them as different
// builds. Sorting here is what makes the cache key honest; without it two people
// asking for the same thing compile it twice.
func CanonicalFlags(flags []string) string {
	out := make([]string, 0, len(flags))
	for _, f := range flags {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

// FirmwareCacheKey identifies a byte-identical build request.
func FirmwareCacheKey(tag, env, canonicalFlags string) string {
	sum := sha256.Sum256([]byte(tag + "|" + env + "|" + canonicalFlags))
	return hex.EncodeToString(sum[:])
}

// FindUsableFirmware returns the newest completed build for this exact request
// whose artifacts have not expired, or nil when there is none.
//
// Expiry is checked in SQL rather than by the reaper alone: an artifact whose
// directory has already been swept must never be handed out just because its row
// still says done.
func (s *Store) FindUsableFirmware(cacheKey, now string) (*FirmwareJob, error) {
	row := s.db.QueryRow(`
		SELECT id, cache_key, tag, env, flags, state, requested_by, error, artifact_dir,
		       created_at, started_at, finished_at, expires_at
		  FROM firmware_jobs
		 WHERE cache_key = ? AND state = ? AND (expires_at IS NULL OR expires_at > ?)
		 ORDER BY created_at DESC LIMIT 1`, cacheKey, FirmwareDone, now)
	j, err := scanFirmwareJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return j, err
}

// FindActiveFirmware returns a queued or building job for this request, so a
// second caller joins the build already in flight instead of starting another.
func (s *Store) FindActiveFirmware(cacheKey string) (*FirmwareJob, error) {
	row := s.db.QueryRow(`
		SELECT id, cache_key, tag, env, flags, state, requested_by, error, artifact_dir,
		       created_at, started_at, finished_at, expires_at
		  FROM firmware_jobs
		 WHERE cache_key = ? AND state IN (?, ?)
		 ORDER BY created_at DESC LIMIT 1`, cacheKey, FirmwareQueued, FirmwareBuilding)
	j, err := scanFirmwareJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return j, err
}

// EnqueueFirmware records a new build request.
func (s *Store) EnqueueFirmware(cacheKey, tag, env, flags string, userID int64, now string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`
		INSERT INTO firmware_jobs (cache_key, tag, env, flags, state, requested_by, created_at)
		VALUES (?,?,?,?,?,?,?)`, cacheKey, tag, env, flags, FirmwareQueued, userID, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetFirmwareJob returns one job by id.
func (s *Store) GetFirmwareJob(id int64) (*FirmwareJob, error) {
	row := s.db.QueryRow(`
		SELECT id, cache_key, tag, env, flags, state, requested_by, error, artifact_dir,
		       created_at, started_at, finished_at, expires_at
		  FROM firmware_jobs WHERE id = ?`, id)
	j, err := scanFirmwareJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return j, err
}

// ClaimNextFirmwareJob moves the oldest queued job to building and returns it.
//
// The UPDATE ... WHERE state = 'queued' is the claim: even though the worker is
// single, this keeps the transition atomic, so a second worker (or a restarted
// one racing the old process) cannot pick up a job already in flight.
func (s *Store) ClaimNextFirmwareJob(now string) (*FirmwareJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var id int64
	err := s.db.QueryRow(`
		SELECT id FROM firmware_jobs WHERE state = ? ORDER BY created_at LIMIT 1`,
		FirmwareQueued).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	res, err := s.db.Exec(`
		UPDATE firmware_jobs SET state = ?, started_at = ? WHERE id = ? AND state = ?`,
		FirmwareBuilding, now, id, FirmwareQueued)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil // lost the race; the next tick will look again
	}
	row := s.db.QueryRow(`
		SELECT id, cache_key, tag, env, flags, state, requested_by, error, artifact_dir,
		       created_at, started_at, finished_at, expires_at
		  FROM firmware_jobs WHERE id = ?`, id)
	return scanFirmwareJob(row)
}

// FinishFirmwareJob records a successful build and when its artifacts expire.
func (s *Store) FinishFirmwareJob(id int64, artifactDir, now string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	expires := time.Now().UTC().Add(ttl).Format(time.RFC3339)
	_, err := s.db.Exec(`
		UPDATE firmware_jobs SET state = ?, artifact_dir = ?, finished_at = ?, expires_at = ?, error = ''
		 WHERE id = ?`, FirmwareDone, artifactDir, now, expires, id)
	return err
}

// FailFirmwareJob records a failed build. The message is stored verbatim for the
// requester; callers are responsible for not putting anything sensitive in it.
func (s *Store) FailFirmwareJob(id int64, msg, now string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		UPDATE firmware_jobs SET state = ?, error = ?, finished_at = ? WHERE id = ?`,
		FirmwareFailed, msg, now, id)
	return err
}

// ExpiredFirmwareJobs lists finished jobs whose artifacts are due for sweeping.
func (s *Store) ExpiredFirmwareJobs(now string) ([]FirmwareJob, error) {
	rows, err := s.db.Query(`
		SELECT id, cache_key, tag, env, flags, state, requested_by, error, artifact_dir,
		       created_at, started_at, finished_at, expires_at
		  FROM firmware_jobs
		 WHERE state = ? AND artifact_dir <> '' AND expires_at IS NOT NULL AND expires_at <= ?`,
		FirmwareDone, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FirmwareJob
	for rows.Next() {
		j, err := scanFirmwareJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

// ForgetFirmwareArtifacts clears the artifact pointer after a sweep, leaving the
// job row as history. The row is kept deliberately: it is the record that a build
// happened and who asked for it, which outlives the file it produced.
func (s *Store) ForgetFirmwareArtifacts(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE firmware_jobs SET artifact_dir = '' WHERE id = ?`, id)
	return err
}

// RequeueStaleFirmwareJobs returns jobs stuck in building back to the queue.
//
// Called at startup: a job left building is one the daemon died in the middle
// of, and nothing will ever finish it. Without this the row sits there forever
// and every later request for the same firmware joins a build that is not
// running.
func (s *Store) RequeueStaleFirmwareJobs() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`
		UPDATE firmware_jobs SET state = ?, started_at = NULL WHERE state = ?`,
		FirmwareQueued, FirmwareBuilding)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CountQueuedFirmware is how many builds are waiting. Used to cap the queue so a
// burst cannot commit the agent to hours of work.
func (s *Store) CountQueuedFirmware() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM firmware_jobs WHERE state = ?`, FirmwareQueued).Scan(&n)
	return n, err
}

// CountFirmwareAhead is how many queued builds sit in front of this one, so a
// waiting requester can be told their place rather than just "please wait".
func (s *Store) CountFirmwareAhead(id int64) (int64, error) {
	var n int64
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM firmware_jobs
		 WHERE state = ? AND id < ?`, FirmwareQueued, id).Scan(&n)
	return n, err
}

// RequeueAbandonedFirmwareJobs returns builds that have been running too long to
// the queue, and reports how many.
//
// Distinct from RequeueStaleFirmwareJobs, which runs once at startup and takes
// every building row because the daemon has just restarted. This one runs
// periodically and must judge by AGE: an agent that died, or one whose result
// was rejected, leaves a job building with nothing left to finish it. Without
// this the row is stuck until the next daemon restart, and every later request
// for that firmware joins a build that is not running.
func (s *Store) RequeueAbandonedFirmwareJobs(cutoff string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`
		UPDATE firmware_jobs SET state = ?, started_at = NULL
		 WHERE state = ? AND (started_at IS NULL OR started_at < ?)`,
		FirmwareQueued, FirmwareBuilding, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RecentFirmwareBuilds lists completed builds for one environment whose
// artifacts are still on disk, newest first.
//
// Deliberately not scoped to the requesting user: the point is that someone
// else's build of the same firmware saves you three minutes. A build is a pure
// function of (release, environment, options) and carries nothing about who
// asked for it.
func (s *Store) RecentFirmwareBuilds(env string, limit int, now string) ([]FirmwareJob, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, cache_key, tag, env, flags, state, requested_by, error, artifact_dir,
		       created_at, started_at, finished_at, expires_at
		  FROM firmware_jobs
		 WHERE env = ? AND state = ? AND artifact_dir <> ''
		   AND (expires_at IS NULL OR expires_at > ?)
		 ORDER BY finished_at DESC LIMIT ?`, env, FirmwareDone, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FirmwareJob
	for rows.Next() {
		j, err := scanFirmwareJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

// rowScanner covers *sql.Row and *sql.Rows so one scan helper serves both.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanFirmwareJob(r rowScanner) (*FirmwareJob, error) {
	var j FirmwareJob
	var started, finished, expires sql.NullString
	var requestedBy sql.NullInt64
	err := r.Scan(&j.ID, &j.CacheKey, &j.Tag, &j.Env, &j.Flags, &j.State, &requestedBy,
		&j.Error, &j.ArtifactDir, &j.CreatedAt, &started, &finished, &expires)
	if err != nil {
		return nil, err
	}
	j.RequestedBy = requestedBy.Int64
	if started.Valid {
		j.StartedAt = &started.String
	}
	if finished.Valid {
		j.FinishedAt = &finished.String
	}
	if expires.Valid {
		j.ExpiresAt = &expires.String
	}
	return &j, nil
}
