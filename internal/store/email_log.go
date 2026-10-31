package store

import "time"

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
