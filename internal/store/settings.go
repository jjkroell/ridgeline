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

// EmergencyEnabled is the admin's switch for the emergency-channel integration
// (SOSChannel): its alerts, alert emails, map rings, monitoring list, guide and
// permanent channel. Off unless an admin turns it on; switched off, the code
// and every stored setting stay, and nothing on the channel triggers anything.
// In memory, so ingest and /api/health check it without the database.
func (s *Store) EmergencyEnabled() bool {
	return s.emergency.Load()
}

// SetEmergencyEnabled records the admin's choice; see EmergencyEnabled.
func (s *Store) SetEmergencyEnabled(on bool, userID int64, now time.Time) error {
	v := "off"
	if on {
		v = "on"
	}
	if _, err := s.db.Exec(`INSERT INTO site_settings (key, value, updated_at, user_id) VALUES ('emergency', ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at, user_id = excluded.user_id`,
		v, now.UTC().Format(time.RFC3339), userID); err != nil {
		return err
	}
	s.emergency.Store(on)
	return nil
}

func (s *Store) loadEmergency() error {
	var v string
	err := s.db.QueryRow(`SELECT value FROM site_settings WHERE key = 'emergency'`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		v, err = "off", nil
	}
	if err != nil {
		return err
	}
	s.emergency.Store(v == "on")
	return nil
}
