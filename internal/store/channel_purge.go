package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

// ChannelObservation is one stored observation of a decrypted channel message,
// as found by ChannelObservations — enough to report and back up before a purge.
type ChannelObservation struct {
	ID          int64  `json:"id"`
	MessageHash string `json:"messageHash"`
	RawHex      string `json:"rawHex"`
	ObserverID  string `json:"observerId"`
	ReceivedAt  string `json:"receivedAt"`
	Sender      string `json:"sender"`
	Message     string `json:"message"`
}

// ChannelObservations returns every stored observation in [now-days, now]
// whose GroupText decrypts to `channel` (a registered name without '#').
// Read-only and batched by the hour over idx_obs_received, so it is safe
// beside a running daemon. The caller registers the channel first.
func (s *Store) ChannelObservations(ctx context.Context, channel string, days int, now time.Time) ([]ChannelObservation, error) {
	var out []ChannelObservation
	end := now.UTC().Truncate(time.Hour).Add(time.Hour)
	for from := end.Add(-time.Duration(days) * 24 * time.Hour); from.Before(end); from = from.Add(time.Hour) {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		rows, err := s.db.Query(`
			SELECT id, message_hash, raw_hex, COALESCE(observer_id,''), received_at FROM observations
			WHERE received_at >= ? AND received_at < ? AND payload_type = 'GroupText'`,
			from.Format(time.RFC3339Nano), from.Add(time.Hour).Format(time.RFC3339Nano))
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var o ChannelObservation
			if err := rows.Scan(&o.ID, &o.MessageHash, &o.RawHex, &o.ObserverID, &o.ReceivedAt); err != nil {
				rows.Close()
				return out, err
			}
			p, err := meshcore.DecodeHex(o.RawHex)
			if err != nil || p == nil || p.GroupText == nil || !p.GroupText.Decrypted || p.GroupText.Channel != channel {
				continue
			}
			o.Sender, o.Message = p.GroupText.Sender, p.GroupText.Message
			out = append(out, o)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return out, err
		}
	}
	return out, nil
}

// DeleteObservations removes observations by id in batches of 500, each a
// short write-only transaction under s.mu (see [[ridgeline-database]]: never
// one transaction over observations). Returns how many rows went.
func (s *Store) DeleteObservations(ids []int64) (int64, error) {
	var total int64
	for len(ids) > 0 {
		n := min(500, len(ids))
		batch := ids[:n]
		ids = ids[n:]
		args := make([]any, len(batch))
		for i, id := range batch {
			args[i] = id
		}
		s.mu.Lock()
		res, err := s.db.Exec(`DELETE FROM observations WHERE id IN (`+
			strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")+`)`, args...)
		s.mu.Unlock()
		if err != nil {
			return total, fmt.Errorf("store: delete observations: %w", err)
		}
		k, _ := res.RowsAffected()
		total += k
		s.obsCount.Add(-k)
	}
	return total, nil
}
