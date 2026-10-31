package store

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// SeasonAuto and SeasonOff are the switch values that aren't a theme id:
// follow each theme's own dates, or no theme at all.
const (
	SeasonAuto = ""
	SeasonOff  = "none"
)

var seasonIDRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// Season is the live map's seasonal-theme switch: SeasonAuto, SeasonOff or the
// id of the one theme turned on (ids live in the web app's SEASONS list).
// In memory, so /api/health serves it without touching the database.
func (s *Store) Season() string {
	v, _ := s.season.Load().(string)
	return v
}

// SetSeason records the admin's choice; see Season.
func (s *Store) SetSeason(v string, userID int64, now time.Time) error {
	if v != SeasonAuto && v != SeasonOff && !seasonIDRe.MatchString(v) {
		return fmt.Errorf("invalid season %q", v)
	}
	if _, err := s.db.Exec(`INSERT INTO site_settings (key, value, updated_at, user_id) VALUES ('season', ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at, user_id = excluded.user_id`,
		v, now.UTC().Format(time.RFC3339), userID); err != nil {
		return err
	}
	s.season.Store(v)
	return nil
}

func (s *Store) loadSeason() error {
	var v string
	err := s.db.QueryRow(`SELECT value FROM site_settings WHERE key = 'season'`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		v, err = SeasonAuto, nil
	}
	if err != nil {
		return err
	}
	s.season.Store(v)
	return nil
}
