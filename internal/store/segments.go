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

// BridgeLink is a sanctioned bridge with both ends known: the near end (the
// relay detection named) and the peer the operator identified as the far side.
type BridgeLink struct {
	Near      string // uppercase pubkey, this side of the wire
	Far       string // uppercase pubkey, the far side
	NearName  string
	FarName   string
	PeerRadio string // operator-declared "freq,bw,sf,cr" of the far segment
}

// FarEnd is the bridge end that sits ON the far segment: the radio transmitting
// the config in PeerRadio. It is the one node for which PeerRadio describes the
// node itself rather than the segment beyond it.
//
// ⚠ TODAY THAT IS Near, NOT Far. Both fields are filled from the blocklist row —
// Near from its key, Far from its peer — and the operator records the
// far-segment end AS the key, so the two names are currently inverted with
// respect to their own doc comments. The direction test in
// analytics.DetectSegments is written to match that inversion, which is the only
// reason the labels have never produced a wrong answer. Ask for an end through
// these accessors rather than naming a field, so correcting the labelling is one
// edit instead of a hunt.
func (l BridgeLink) FarEnd() string { return l.Near }

// NearEnd is the bridge end on this side of the wire. See FarEnd.
func (l BridgeLink) NearEnd() string { return l.Far }

// KnownBridgeLinks returns sanctioned bridges that have a peer recorded. A
// known bridge with no peer can't define a segment — a link needs two ends —
// so it is skipped rather than guessed at.
func (s *Store) KnownBridgeLinks() ([]BridgeLink, error) {
	rows, err := s.db.Query(`
		SELECT b.key, b.peer, COALESCE(b.name,''), COALESCE(nf.name,''), COALESCE(b.peer_radio,'')
		FROM blocklist b
		LEFT JOIN nodes nf ON UPPER(nf.pubkey) = b.peer
		WHERE b.kind = ? AND b.peer IS NOT NULL AND b.peer <> ''`, BlockKnown)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BridgeLink{}
	for rows.Next() {
		var l BridgeLink
		if err := rows.Scan(&l.Near, &l.Far, &l.NearName, &l.FarName, &l.PeerRadio); err != nil {
			return nil, err
		}
		l.Near, l.Far = strings.ToUpper(l.Near), strings.ToUpper(l.Far)
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
	NodeKey    string // uppercase pubkey
	BridgeNear string // uppercase pubkey of the bridge it is reached through
	Confidence string // "confirmed" | "probable"
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
			strings.ToUpper(m.BridgeNear), m.Confidence, strings.ToUpper(m.NodeKey))
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
