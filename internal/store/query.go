package store

// Node is a row from the nodes table, shaped for API responses.
type Node struct {
	PublicKey   string   `json:"publicKey"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	HasLocation bool     `json:"hasLocation"`
	FirstSeen   string   `json:"firstSeen"`
	LastSeen    string   `json:"lastSeen"`
	LastAdvert  string   `json:"lastAdvert,omitempty"`
	AdvertCount int      `json:"advertCount"`
	// HashSize is the node's path-hash length in bytes (1, 2, or 3), learned
	// from its advert. 0 means not yet known.
	HashSize int `json:"hashSize"`
}

// Stats is a high-level snapshot of the database.
type Stats struct {
	Nodes        int    `json:"nodes"`
	Observers    int    `json:"observers"`
	Observations int    `json:"observations"`
	LastPacketAt string `json:"lastPacketAt,omitempty"`
}

// ListNodes returns all known nodes, most recently seen first.
func (s *Store) ListNodes() ([]Node, error) {
	rows, err := s.db.Query(`
		SELECT pubkey, COALESCE(name,''), COALESCE(role,''),
		       latitude, longitude, has_location,
		       first_seen, last_seen, COALESCE(last_advert,''), advert_count,
		       COALESCE(hash_size, 0)
		FROM nodes
		ORDER BY last_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := []Node{}
	for rows.Next() {
		var n Node
		var hasLoc int
		if err := rows.Scan(&n.PublicKey, &n.Name, &n.Role,
			&n.Latitude, &n.Longitude, &hasLoc,
			&n.FirstSeen, &n.LastSeen, &n.LastAdvert, &n.AdvertCount, &n.HashSize); err != nil {
			return nil, err
		}
		n.HasLocation = hasLoc != 0
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// Stats returns counts and the most recent packet time.
func (s *Store) Stats() (Stats, error) {
	var st Stats
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&st.Nodes); err != nil {
		return st, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM observers`).Scan(&st.Observers); err != nil {
		return st, err
	}
	var last *string
	if err := s.db.QueryRow(
		`SELECT COUNT(*), MAX(received_at) FROM observations`,
	).Scan(&st.Observations, &last); err != nil {
		return st, err
	}
	if last != nil {
		st.LastPacketAt = *last
	}
	return st, nil
}

// Observer is a row from the observers table. Latitude/Longitude are resolved
// by matching the observer's public key to an advertised node, when available.
type Observer struct {
	ID          string   `json:"id"`
	Region      string   `json:"region"`
	PublicKey   string   `json:"publicKey,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	FirstSeen   string   `json:"firstSeen"`
	LastSeen    string   `json:"lastSeen"`
	PacketCount int      `json:"packetCount"`
}

// ListObservers returns all observers, most recently active first, with a
// location joined from the nodes table when the observer's key has advertised.
func (s *Store) ListObservers() ([]Observer, error) {
	rows, err := s.db.Query(`
		SELECT o.id, COALESCE(o.region,''), COALESCE(o.pubkey,''),
		       n.latitude, n.longitude, o.first_seen, o.last_seen, o.packet_count
		FROM observers o
		LEFT JOIN nodes n ON n.pubkey = o.pubkey
		ORDER BY o.last_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Observer{}
	for rows.Next() {
		var o Observer
		if err := rows.Scan(&o.ID, &o.Region, &o.PublicKey,
			&o.Latitude, &o.Longitude, &o.FirstSeen, &o.LastSeen, &o.PacketCount); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// RecentObservation is a lightweight view of a recent packet sighting.
type RecentObservation struct {
	MessageHash string   `json:"messageHash"`
	RouteType   string   `json:"routeType"`
	PayloadType string   `json:"payloadType"`
	PathHops    int      `json:"pathHops"`
	ObserverID  string   `json:"observerId,omitempty"`
	Region      string   `json:"region,omitempty"`
	SNR         *float64 `json:"snr,omitempty"`
	RSSI        *float64 `json:"rssi,omitempty"`
	ReceivedAt  string   `json:"receivedAt"`
}

// RecentObservations returns the most recent observations, newest first.
func (s *Store) RecentObservations(limit int) ([]RecentObservation, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT message_hash, route_type, payload_type, path_hops,
		       COALESCE(observer_id,''), COALESCE(region,''), snr, rssi, received_at
		FROM observations
		ORDER BY id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RecentObservation{}
	for rows.Next() {
		var o RecentObservation
		if err := rows.Scan(&o.MessageHash, &o.RouteType, &o.PayloadType, &o.PathHops,
			&o.ObserverID, &o.Region, &o.SNR, &o.RSSI, &o.ReceivedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
