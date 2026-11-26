package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// SurveyResponse is one stored submission (see api/survey.go). Group is the
// first characters of the sender's salted IP hash: answers that share it came
// from the same connection (a household or a mobile carrier can share one too).
type SurveyResponse struct {
	ID        int64           `json:"id"`
	Answers   json.RawMessage `json:"answers"`
	CreatedAt string          `json:"createdAt"`
	UserID    int64           `json:"userId,omitempty"`
	Group     string          `json:"group,omitempty"`
}

// SurveyIPHash turns an IP address into the value stored with an answer: a
// SHA-256 over a random per-instance salt (site_settings "survey_salt", made on
// first use) and the address. The address itself is never stored, and without
// the salt the hash can't be matched against a list of addresses.
func (s *Store) SurveyIPHash(ip string) (string, error) {
	var salt string
	err := s.db.QueryRow(`SELECT value FROM site_settings WHERE key = 'survey_salt'`).Scan(&salt)
	if errors.Is(err, sql.ErrNoRows) {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		// INSERT OR IGNORE then re-read: two first answers at once agree on one salt.
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO site_settings (key, value, updated_at, user_id) VALUES ('survey_salt', ?, ?, 0)`,
			hex.EncodeToString(b), time.Now().UTC().Format(time.RFC3339)); err != nil {
			return "", err
		}
		err = s.db.QueryRow(`SELECT value FROM site_settings WHERE key = 'survey_salt'`).Scan(&salt)
	}
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(salt + "|" + ip))
	return hex.EncodeToString(sum[:]), nil
}

// AddSurveyResponse stores one submission; answers is already-validated JSON.
func (s *Store) AddSurveyResponse(survey string, answers []byte, userID int64, ipHash string, now time.Time) error {
	_, err := s.db.Exec(`INSERT INTO survey_responses (survey, answers, created_at, user_id, ip_hash) VALUES (?, ?, ?, ?, ?)`,
		survey, string(answers), now.UTC().Format(time.RFC3339), userID, ipHash)
	return err
}

// SurveyResponses returns a survey's submissions, newest first. A survey is a
// few hundred rows at most, so it is read whole.
func (s *Store) SurveyResponses(survey string) ([]SurveyResponse, error) {
	rows, err := s.db.Query(`SELECT id, answers, created_at, user_id, ip_hash FROM survey_responses
		WHERE survey = ? ORDER BY id DESC`, survey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SurveyResponse{}
	for rows.Next() {
		var r SurveyResponse
		var a, h string
		if err := rows.Scan(&r.ID, &a, &r.CreatedAt, &r.UserID, &h); err != nil {
			return nil, err
		}
		r.Answers = json.RawMessage(a)
		if len(h) >= 6 {
			r.Group = h[:6]
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
