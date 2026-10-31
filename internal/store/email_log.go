package store

import (
	"strings"
	"time"
)

// emailLogKeep is how long email_log rows are kept.
const emailLogKeep = 365 * 24 * time.Hour

// LogEmail records one email handed to the SMTP relay; sendErr is what the
// relay said if it refused (nil: accepted).
func (s *Store) LogEmail(kind, to, subject string, sendErr error) error {
	msg := ""
	if sendErr != nil {
		msg = sendErr.Error()
	}
	_, err := s.db.Exec(`INSERT INTO email_log (sent_at, kind, recipient, subject, error)
		VALUES (?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339Nano), kind, to, subject, msg)
	return err
}

// PruneEmailLog deletes email_log rows older than a year.
func (s *Store) PruneEmailLog() (int64, error) {
	cutoff := time.Now().Add(-emailLogKeep).UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`DELETE FROM email_log WHERE sent_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// EmailLogEntry is one email_log row.
type EmailLogEntry struct {
	ID        int64  `json:"id"`
	SentAt    string `json:"sentAt"`
	Kind      string `json:"kind"`
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Error     string `json:"error,omitempty"`
}

// ListEmailLog returns up to limit emails, newest first, older than id before
// (0: from the newest). kind filters to one kind; q matches part of the
// recipient address. more reports whether older matching rows remain.
func (s *Store) ListEmailLog(kind, q string, before int64, limit int) (out []EmailLogEntry, more bool, err error) {
	where, args := `WHERE 1=1`, []any{}
	if before > 0 {
		where += ` AND id < ?`
		args = append(args, before)
	}
	if kind != "" {
		where += ` AND kind = ?`
		args = append(args, kind)
	}
	if q = strings.TrimSpace(q); q != "" {
		where += ` AND instr(recipient, ?) > 0`
		args = append(args, strings.ToLower(q))
	}
	rows, err := s.db.Query(`SELECT id, sent_at, kind, recipient, subject, error FROM email_log `+
		where+` ORDER BY id DESC LIMIT ?`, append(args, limit+1)...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out = []EmailLogEntry{}
	for rows.Next() {
		var e EmailLogEntry
		if err := rows.Scan(&e.ID, &e.SentAt, &e.Kind, &e.Recipient, &e.Subject, &e.Error); err != nil {
			return nil, false, err
		}
		out = append(out, e)
	}
	if len(out) > limit {
		out, more = out[:limit], true
	}
	return out, more, rows.Err()
}
