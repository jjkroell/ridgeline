package store

// Network segments behind sanctioned bridges.
//
// A bridge joins two RF networks. Where no receiver sits on the far side, the
// nodes over there are only ever heard after their traffic crosses the wire — so
// "reached only across bridge X" is the sole evidence that a node lives on the
// far segment, and the far side's radio settings cannot be observed at all.
//
// Once a receiver IS added on the far segment (see analytics.DetectSegments) its
// direct receptions become better evidence than any of that, and membership is
// recorded with confidence "observed". The declared radio below is then
// corroborated rather than unverifiable — but it stays the value reported,
// because it describes the segment while a node's inherited radio still
// describes whichever receiver last heard it.
//
// That last point is why BridgeLink carries an operator-declared Radio. A
// far-side node's nodes.radio is inherited from whichever observer heard it,
// which is a receiver on THIS side, so it describes the listener rather than the
// node. The API suppresses it for far-side nodes and reports the declared value
// instead.

import "strings"

// BridgeLink is a sanctioned bridge with both of its ends known.
//
// ★ Near and Far are PHYSICAL positions, not column names. Near is the end on
// THIS side of the wire, the segment nearly all our receivers sit on; Far is the
// end transmitting on the segment PeerRadio describes. Key is separate from
// both: it is the bridge's identity, what nodes.via_bridge stores, and what the
// admin console shows — so it stays stable no matter which end is which.
//
// The distinction is load-bearing. Every conclusion this package draws comes
// from the ORDER the two ends appear in a relay path, and reading that order
// against the wrong end inverts the result silently: nodes on the far segment
// stop being found and the feature reports an empty far side rather than an
// error. KnownBridgeLinks documents how the ends are assigned.
type BridgeLink struct {
	Key       string // blocklist key: the bridge's identity (nodes.via_bridge)
	Name      string // the bridge's name as the operator recorded it
	Near      string // uppercase pubkey of the end on THIS segment
	Far       string // uppercase pubkey of the end on the FAR segment
	NearName  string
	FarName   string
	PeerRadio string // operator-declared "freq,bw,sf,cr" of the far segment
}

// FarEnd is the bridge end that sits ON the far segment: the radio transmitting
// the config in PeerRadio. It is the one node for which PeerRadio describes the
// node itself rather than the segment beyond it.
func (l BridgeLink) FarEnd() string { return l.Far }

// NearEnd is the bridge end on this side of the wire. See FarEnd.
func (l BridgeLink) NearEnd() string { return l.Near }

// KnownBridgeLinks returns sanctioned bridges that have a peer recorded. A
// known bridge with no peer can't define a segment — a link needs two ends —
// so it is skipped rather than guessed at.
//
// ⚠ WHICH COLUMN HOLDS WHICH END. The row's KEY is the end on the far segment
// and its PEER is the end on this side. That is the opposite of what the column
// names suggest, and it is a convention about how a bridge gets RECORDED rather
// than anything derived from the traffic: the operator sanctions the end that
// detection surfaced — the one whose relaying looked foreign, which is the end
// living over there — and then names the end it is wired to over here.
//
// It cannot be checked at this layer, so it is checked one layer up instead:
// DetectSegments counts crossings in both directions and reports the bridge in
// SegmentReport.ReversedEnds when the traffic runs mostly the wrong way, which
// is what a bridge recorded end-for-end looks like. Prefer that signal over
// trusting this comment.
func (s *Store) KnownBridgeLinks() ([]BridgeLink, error) {
	rows, err := s.db.Query(`
		SELECT b.key, b.peer, COALESCE(b.name,''),
		       COALESCE(nk.name,''), COALESCE(np.name,''), COALESCE(b.peer_radio,'')
		FROM blocklist b
		LEFT JOIN nodes nk ON UPPER(nk.pubkey) = b.key
		LEFT JOIN nodes np ON UPPER(np.pubkey) = b.peer
		WHERE b.kind = ? AND b.peer IS NOT NULL AND b.peer <> ''`, BlockKnown)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BridgeLink{}
	for rows.Next() {
		var l BridgeLink
		// key -> Far, peer -> Near: see the note above.
		if err := rows.Scan(&l.Far, &l.Near, &l.Name, &l.FarName, &l.NearName, &l.PeerRadio); err != nil {
			return nil, err
		}
		l.Near, l.Far = strings.ToUpper(l.Near), strings.ToUpper(l.Far)
		l.Key = l.Far
		out = append(out, l)
	}
	return out, rows.Err()
}

// SetBridgePeerRadio records the far segment's radio config on a sanctioned
// bridge. Empty clears it.
func (s *Store) SetBridgePeerRadio(key, radio string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE blocklist SET peer_radio = ? WHERE kind = ? AND key = ?`,
		nullStr(strings.TrimSpace(radio)), BlockKnown, strings.ToUpper(key))
	return err
}

// SegmentMember is one node found to live beyond a bridge.
type SegmentMember struct {
	NodeKey string // uppercase pubkey
	// BridgeKey is the bridge's identity (BridgeLink.Key), NOT either of its
	// ends. Naming an end here would move this value whenever the labelling of
	// the ends changed, and it is stored in nodes.via_bridge and rendered in the
	// admin console.
	BridgeKey  string
	Confidence string // "observed" | "confirmed" | "probable"
}

// ApplySegments replaces the whole far-side assignment in one transaction.
//
// Replace rather than merge: membership is a conclusion drawn from a rolling
// window, so a node that stops crossing must stop being labelled. Merging would
// make the label sticky and it would slowly drift out of agreement with the
// traffic — the same failure mode as a cache nobody invalidates.
func (s *Store) ApplySegments(members []SegmentMember) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE nodes SET via_bridge = NULL, via_bridge_conf = NULL WHERE via_bridge IS NOT NULL`); err != nil {
		return 0, err
	}
	n := 0
	for _, m := range members {
		res, err := tx.Exec(`UPDATE nodes SET via_bridge = ?, via_bridge_conf = ? WHERE UPPER(pubkey) = ?`,
			strings.ToUpper(m.BridgeKey), m.Confidence, strings.ToUpper(m.NodeKey))
		if err != nil {
			return 0, err
		}
		if c, _ := res.RowsAffected(); c > 0 {
			n++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}
