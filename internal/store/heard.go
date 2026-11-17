package store

import (
	"database/sql"
	"strings"
	"time"
)

// heardThrottle is the minimum gap between two last_heard writes for one key.
// One discover reply reaches several observers within a second, and a minute's
// precision is all "last heard" needs.
const heardThrottle = time.Minute

// minHeardPrefixHex is the shortest responder key a discover reply may carry
// and still be credited: 8 bytes (16 hex). MeshCore replies with either the
// full 32-byte key or an 8-byte prefix; at 8 bytes a collision between two
// real nodes is not a practical concern, but anything shorter could be.
const minHeardPrefixHex = 16

// noteDiscoverHeard records that the node behind a node-discover reply's
// responder key was heard at ts. It updates the node row when there is one,
// and otherwise the claim on that key (a dormant claim, whose node row
// retention has removed). A prefix key is credited only when it matches
// exactly one node (or claim). Caller holds s.mu and passes the open tx.
//
// Every lookup is a primary-key or idx_claims_node range — never a scan.
func (s *Store) noteDiscoverHeard(tx *sql.Tx, key, ts string, at time.Time) error {
	key = strings.ToUpper(strings.TrimSpace(key))
	if len(key) < minHeardPrefixHex || len(key) > 64 || !isHex(key) {
		return nil
	}
	if last, ok := s.heardWritten[key]; ok && at.Sub(last) < heardThrottle && at.After(last) {
		return nil
	}
	if s.heardWritten == nil {
		s.heardWritten = map[string]time.Time{}
	}
	s.heardWritten[key] = at

	nodeKey, err := uniqueKeyByPrefix(tx, `SELECT pubkey FROM nodes WHERE pubkey >= ? AND pubkey < ? LIMIT 2`, key)
	if err != nil {
		return err
	}
	if nodeKey != "" {
		_, err := tx.Exec(`UPDATE nodes SET last_heard = ? WHERE pubkey = ? AND COALESCE(last_heard,'') < ?`,
			ts, nodeKey, ts)
		return err
	}
	claimKey, err := uniqueKeyByPrefix(tx, `SELECT DISTINCT node_pubkey FROM node_claims WHERE node_pubkey >= ? AND node_pubkey < ? LIMIT 2`, key)
	if err != nil || claimKey == "" {
		return err
	}
	_, err = tx.Exec(`UPDATE node_claims SET last_heard = ? WHERE node_pubkey = ? AND COALESCE(last_heard,'') < ?`,
		ts, claimKey, ts)
	return err
}

// uniqueKeyByPrefix runs a two-row range query for keys starting with prefix
// (upper-case hex, so prefix+"G" sorts after every continuation) and returns
// the key only when exactly one matches.
func uniqueKeyByPrefix(tx *sql.Tx, query, prefix string) (string, error) {
	rows, err := tx.Query(query, prefix, prefix+"G")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var found []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return "", err
		}
		found = append(found, k)
	}
	if err := rows.Err(); err != nil || len(found) != 1 {
		return "", err
	}
	return found[0], nil
}

func isHex(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
