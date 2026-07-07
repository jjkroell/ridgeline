package store

import (
	"strings"
	"time"
)

// LocationShare is one grant of read access to a node's private location. The
// display fields identify the grantee for the owner's management UI.
type LocationShare struct {
	NodePubkey    string `json:"nodePubkey"`
	GranteeUserID int64  `json:"granteeUserId"`
	DisplayName   string `json:"displayName"`
	Email         string `json:"email"`
	CreatedAt     string `json:"createdAt"`
}

// ShareLocation grants granteeUserID read access to nodePubkey's private
// location. Idempotent (re-granting just refreshes the timestamp). Ownership is
// verified by the caller (API layer) before this is invoked.
func (s *Store) ShareLocation(nodePubkey string, ownerUserID, granteeUserID int64) error {
	nodePubkey = strings.ToUpper(nodePubkey)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`
		INSERT INTO location_shares (node_pubkey, owner_user_id, grantee_user_id, created_at)
		VALUES (?,?,?,?)
		ON CONFLICT(node_pubkey, grantee_user_id) DO UPDATE SET
			owner_user_id = excluded.owner_user_id,
			created_at    = excluded.created_at`,
		nodePubkey, ownerUserID, granteeUserID, now)
	return err
}

// UnshareLocation revokes a grantee's access. Returns whether a row was removed.
func (s *Store) UnshareLocation(nodePubkey string, granteeUserID int64) (bool, error) {
	nodePubkey = strings.ToUpper(nodePubkey)
	res, err := s.db.Exec(`DELETE FROM location_shares WHERE node_pubkey = ? AND grantee_user_id = ?`,
		nodePubkey, granteeUserID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// HasLocationShare reports whether userID has been granted read access to a
// node's private location.
func (s *Store) HasLocationShare(nodePubkey string, userID int64) (bool, error) {
	nodePubkey = strings.ToUpper(nodePubkey)
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM location_shares WHERE node_pubkey = ? AND grantee_user_id = ?`,
		nodePubkey, userID).Scan(&n)
	return n > 0, err
}

// ListLocationShares returns a node's grantees with their display info, newest
// grant first.
func (s *Store) ListLocationShares(nodePubkey string) ([]LocationShare, error) {
	nodePubkey = strings.ToUpper(nodePubkey)
	rows, err := s.db.Query(`
		SELECT ls.node_pubkey, ls.grantee_user_id,
		       COALESCE(NULLIF(u.display_name,''), u.email), u.email, ls.created_at
		FROM location_shares ls JOIN users u ON u.id = ls.grantee_user_id
		WHERE ls.node_pubkey = ?
		ORDER BY ls.created_at DESC`, nodePubkey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LocationShare
	for rows.Next() {
		var sh LocationShare
		if err := rows.Scan(&sh.NodePubkey, &sh.GranteeUserID, &sh.DisplayName, &sh.Email, &sh.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// DeleteLocationShares drops all of a node's shares (used when the owner
// releases the node).
func (s *Store) DeleteLocationShares(nodePubkey string) error {
	nodePubkey = strings.ToUpper(nodePubkey)
	_, err := s.db.Exec(`DELETE FROM location_shares WHERE node_pubkey = ?`, nodePubkey)
	return err
}
