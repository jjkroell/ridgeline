package store

import (
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

// Block kinds.
const (
	BlockObserver = "observer" // rogue MQTT publisher (observer id)
	BlockBridge   = "bridge"   // RF bridge node (pubkey) — drops anything it relays
	BlockNode     = "node"     // a single injected node (pubkey) — drops its own adverts
	BlockAllow    = "allow"    // dismissed candidate (pubkey) — excluded from detection, NOT blocked
	// BlockKnown marks a SANCTIONED bridge (pubkey): a real bridge the operator
	// runs on purpose. Nothing is blocked and nothing is hidden — unlike "allow",
	// which asserts a candidate is not a bridge, this asserts it is one and that
	// it is wanted. Detection still reports it, labelled, so it stops reading as
	// a finding that needs acting on every single scan.
	BlockKnown = "known"

	// BlockLink blocks a DIRECTED relay hop PAIR rather than a node: key is the
	// hop traffic arrived on, peer is the hop it was relayed to. A packet drops
	// only when those two appear ADJACENT, in that order, in its relay path.
	// That is what kills the traffic an RF bridge injects without touching
	// either end's own traffic — and the far end is usually a legitimate
	// repeater on this mesh, so blocking it outright is not an option.
	//
	// Why not BlockBridge for this: a path hop is a truncated hash whose width
	// is chosen by the packet's ORIGINATOR (1, 2, or 3 bytes), so a single-key
	// prefix block fires for every node sharing that prefix. Blocking Cokley 425
	// (22563EB8…) that way also drops traffic relayed by MT BRENTON CVARS
	// (2235D8…) and VA7NEX (22A2E7…) whenever a path carries 1-byte hops —
	// measured over 17h of real traffic as ~93% of all its matches. Requiring
	// the pair resolves the ambiguity without a width floor: across 13 days of
	// certain-identity (>=2-byte) hops the colliding candidates were never once
	// adjacent to each other, only the real bridge pair ever was.
	BlockLink = "link"
)

// blockedLink is a directed relay hop pair held in the ingest cache. From and To
// are full pubkeys; relay path hops are truncated hashes, so each end matches by
// prefix at whatever width the originator used.
type blockedLink struct{ From, To string }

// pubkeyKind reports whether a block kind's key is a node pubkey (so it should be
// stored/compared upper-cased) rather than a free-form observer id.
func pubkeyKind(kind string) bool {
	return kind == BlockNode || kind == BlockBridge || kind == BlockAllow ||
		kind == BlockKnown || kind == BlockLink
}

// BlockEntry is one blocklist row.
type BlockEntry struct {
	Kind      string `json:"kind"`
	Key       string `json:"key"`
	Name      string `json:"name,omitempty"`
	Reason    string `json:"reason,omitempty"`
	CreatedAt string `json:"createdAt"`
	// Peer is the node on the FAR side of a sanctioned bridge (BlockKnown only):
	// the neighbour this bridge carries traffic to. Stored as an uppercase pubkey
	// because a bridge is a link between two identities and a name is only a
	// label — the operator can rename either end without breaking the record.
	Peer string `json:"peer,omitempty"`
	// PeerName is Peer's current display name, resolved at read time rather than
	// captured at write time so a rename follows the node.
	PeerName string `json:"peerName,omitempty"`
}

// loadBlocklist refreshes the in-memory blocklist cache from the table.
func (s *Store) loadBlocklist() error {
	rows, err := s.db.Query(`SELECT kind, key, COALESCE(peer,'') FROM blocklist`)
	if err != nil {
		return err
	}
	defer rows.Close()
	obs := map[string]bool{}
	nodes := map[string]bool{}
	allow := map[string]bool{}
	known := map[string]bool{}
	var bridges []string
	var links []blockedLink
	for rows.Next() {
		var kind, key, peer string
		if err := rows.Scan(&kind, &key, &peer); err != nil {
			return err
		}
		switch kind {
		case BlockObserver:
			obs[key] = true
		case BlockBridge:
			k := strings.ToUpper(key)
			nodes[k] = true
			bridges = append(bridges, k)
		case BlockNode:
			nodes[strings.ToUpper(key)] = true
		case BlockAllow:
			allow[strings.ToUpper(key)] = true
		case BlockKnown:
			known[strings.ToUpper(key)] = true
		case BlockLink:
			// A link with no far end names no pair and can only match loosely,
			// so it stays inert rather than degrading to a single-key block.
			if peer != "" {
				links = append(links, blockedLink{
					From: strings.ToUpper(key),
					To:   strings.ToUpper(peer),
				})
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.blockMu.Lock()
	s.blockedObservers, s.blockedNodes, s.blockedBridges, s.allowedNodes = obs, nodes, bridges, allow
	s.knownBridges = known
	s.blockedLinks = links
	s.blockMu.Unlock()
	return nil
}

// ShouldDrop reports whether an incoming observation is blocklisted and must not
// be ingested. Called on the hot path, so it only reads the cache. It drops:
//   - anything published by a blocked observer,
//   - an advert originated by a blocked node or bridge,
//   - any packet whose flood path transits a blocked bridge (a hop prefix of the
//     bridge's pubkey) — this is what kills the foreign traffic an RF bridge injects,
//   - any packet whose flood path relayed across a blocked LINK: two hops
//     adjacent and in order. See BlockLink for why a pair is matched rather
//     than one end.
func (s *Store) ShouldDrop(p *meshcore.Packet, observerID string) bool {
	if p == nil {
		return false
	}
	s.blockMu.RLock()
	defer s.blockMu.RUnlock()
	if len(s.blockedObservers) == 0 && len(s.blockedNodes) == 0 &&
		len(s.blockedBridges) == 0 && len(s.blockedLinks) == 0 {
		return false
	}
	if observerID != "" && s.blockedObservers[observerID] {
		return true
	}
	if p.Advert != nil && p.Advert.PublicKey != "" && s.blockedNodes[strings.ToUpper(p.Advert.PublicKey)] {
		return true
	}
	path := p.RelayPath()
	for _, hop := range path {
		if hop == "" {
			continue
		}
		h := strings.ToUpper(hop)
		for _, b := range s.blockedBridges {
			if strings.HasPrefix(b, h) {
				return true
			}
		}
	}
	// A blocked link matches only where its two ends are adjacent and in order:
	// the direction is the whole point, since the reverse is this mesh's own
	// traffic on its way out rather than foreign traffic on its way in.
	for i := 0; i+1 < len(path); i++ {
		from, to := strings.ToUpper(path[i]), strings.ToUpper(path[i+1])
		if from == "" || to == "" {
			continue
		}
		for _, l := range s.blockedLinks {
			if strings.HasPrefix(l.From, from) && strings.HasPrefix(l.To, to) {
				return true
			}
		}
	}
	return false
}

// IsNodeBlocked reports whether a node/bridge pubkey is blocklisted (for hiding
// it from public API responses).
func (s *Store) IsNodeBlocked(pubkey string) bool {
	s.blockMu.RLock()
	defer s.blockMu.RUnlock()
	return s.blockedNodes[strings.ToUpper(pubkey)]
}

// NodeBlock returns the blocklist entry that quarantines a node/bridge pubkey,
// or nil if it isn't blocked. Prefers a "bridge" entry over a plain "node" one.
func (s *Store) NodeBlock(pubkey string) *BlockEntry {
	var e BlockEntry
	err := s.db.QueryRow(`
		SELECT kind, key, COALESCE(name,''), COALESCE(reason,''), created_at
		FROM blocklist
		WHERE key = ? AND kind IN ('node','bridge')
		ORDER BY CASE kind WHEN 'bridge' THEN 0 ELSE 1 END
		LIMIT 1`, strings.ToUpper(pubkey)).Scan(&e.Kind, &e.Key, &e.Name, &e.Reason, &e.CreatedAt)
	if err != nil {
		return nil
	}
	return &e
}

// IsAllowed reports whether a node pubkey has been dismissed as a detection
// candidate (allowlisted). Such nodes are excluded from injection detection but
// are NOT blocked.
func (s *Store) IsAllowed(pubkey string) bool {
	s.blockMu.RLock()
	defer s.blockMu.RUnlock()
	return s.allowedNodes[strings.ToUpper(pubkey)]
}

// AddBlock inserts (or updates) a blocklist entry and refreshes the cache.
func (s *Store) AddBlock(kind, key, name, reason string) error {
	return s.AddBlockPeer(kind, key, name, reason, "")
}

// AddBlockPeer is AddBlock with the far side of a sanctioned bridge recorded.
// peer is only meaningful for BlockKnown; passing it for any other kind stores a
// value nothing reads. An empty peer leaves any existing one intact, so
// re-marking a bridge doesn't silently forget which link it is.
func (s *Store) AddBlockPeer(kind, key, name, reason, peer string) error {
	if pubkeyKind(kind) {
		key = strings.ToUpper(key)
	}
	peer = strings.ToUpper(strings.TrimSpace(peer))
	// A bridge cannot be its own far side; storing that would render "X → X".
	if peer == key {
		peer = ""
	}
	s.mu.Lock()
	_, err := s.db.Exec(`
		INSERT INTO blocklist (kind, key, name, reason, created_at, peer)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(kind, key) DO UPDATE SET
			name   = COALESCE(NULLIF(excluded.name,''), blocklist.name),
			reason = COALESCE(NULLIF(excluded.reason,''), blocklist.reason),
			peer   = COALESCE(NULLIF(excluded.peer,''), blocklist.peer)`,
		kind, key, nullStr(name), nullStr(reason), time.Now().UTC().Format(time.RFC3339), nullStr(peer))
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.loadBlocklist()
}

// ClearBlockPeer forgets the far side of a sanctioned bridge without unmarking
// it. AddBlockPeer deliberately treats an empty peer as "leave alone", so
// changing a link from "X → Y" back to "X → unknown" needs its own call.
func (s *Store) ClearBlockPeer(kind, key string) error {
	if pubkeyKind(kind) {
		key = strings.ToUpper(key)
	}
	s.mu.Lock()
	_, err := s.db.Exec(`UPDATE blocklist SET peer = NULL WHERE kind = ? AND key = ?`, kind, key)
	s.mu.Unlock()
	return err
}

// RemoveBlock deletes a blocklist entry (un-quarantine) and refreshes the cache.
func (s *Store) RemoveBlock(kind, key string) error {
	if pubkeyKind(kind) {
		key = strings.ToUpper(key)
	}
	s.mu.Lock()
	_, err := s.db.Exec(`DELETE FROM blocklist WHERE kind = ? AND key = ?`, kind, key)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.loadBlocklist()
}

// ListBlocks returns all blocklist entries, newest first. A sanctioned bridge's
// peer name is joined from the nodes table at read time (see BlockEntry.PeerName)
// so a renamed far-end node shows its current name rather than the one it had
// when the link was recorded.
func (s *Store) ListBlocks() ([]BlockEntry, error) {
	rows, err := s.db.Query(`
		SELECT b.kind, b.key, COALESCE(b.name,''), COALESCE(b.reason,''), b.created_at,
		       COALESCE(b.peer,''), COALESCE(n.name,'')
		FROM blocklist b
		LEFT JOIN nodes n ON UPPER(n.pubkey) = b.peer
		ORDER BY b.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BlockEntry{}
	for rows.Next() {
		var e BlockEntry
		if err := rows.Scan(&e.Kind, &e.Key, &e.Name, &e.Reason, &e.CreatedAt, &e.Peer, &e.PeerName); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PurgeResult reports what a purge removed.
type PurgeResult struct {
	Observations int64 `json:"observations"`
	// Telemetry counts observer device-telemetry rows removed alongside a
	// deleted observer.
	Telemetry int64 `json:"telemetry"`
	Nodes     int64 `json:"nodes"`
	// OrphanNodes counts the subset of Nodes that was not named by the caller:
	// node rows whose every stored advert came from an observer being removed,
	// so nothing outside that observer's feed ever evidenced them. They are
	// included in Nodes as well.
	OrphanNodes int64 `json:"orphanNodes"`
	Claims      int64 `json:"claims"`
	Notes       int64 `json:"notes"`
	Locations   int64 `json:"locations"`
	Shares      int64 `json:"shares"`
	// SkippedClaimed lists keys held back from the delete because a user has
	// claimed them — set by the purge handler for the nodes IT was asked to
	// remove, and by the store for orphans it found on its own.
	SkippedClaimed []string `json:"skippedClaimed,omitempty"`
}

// PurgeTargets hard-deletes stored data for the given targets in a single
// observation scan: observations published by any observer in observers,
// observations whose flood path transits any bridge pubkey in bridges, and the
// own adverts + node rows of any pubkey in nodes (and bridges). Pubkeys are
// matched case-insensitively; bridge path matching is by hash-prefix.
//
// Removing an observer also removes the node rows that ONLY it evidenced — see
// the orphan pass below.
//
// User-authored data keyed to the node (claims, notes, private location,
// location shares) is PRESERVED. This is the path the automatic retention sweep
// uses, where a node pruned for going silent is expected to come back on its
// next advert — an operator who takes a repeater down for a week must not lose
// their ownership claim or private location. Use ScrubNodes for deliberate
// admin removal, which does cascade.
func (s *Store) PurgeTargets(observers, bridges, nodes []string) (PurgeResult, error) {
	return s.purgeTargets(observers, bridges, nodes, false)
}

// ScrubNodes hard-deletes nodes AND the user-authored data keyed to them
// (claims, notes, private location, location shares). This is the admin scrub /
// purge path, where removal is deliberate and permanent — the node is bogus,
// foreign, or injected, so its claim should not survive. Leaving the claim
// behind orphaned it: it still rendered in "Claimed Nodes" and on badges
// pointing at a node that no longer exists, and — because idx_claims_one_owner
// is unique per node — it would block the node from ever being re-claimed if it
// advertised again.
func (s *Store) ScrubNodes(observers, bridges, nodes []string) (PurgeResult, error) {
	return s.purgeTargets(observers, bridges, nodes, true)
}

func (s *Store) purgeTargets(observers, bridges, nodes []string, cascadeUserData bool) (PurgeResult, error) {
	var res PurgeResult
	obsSet := map[string]bool{}
	for _, o := range observers {
		obsSet[o] = true
	}
	bridgeSet := make([]string, 0, len(bridges))
	for _, b := range bridges {
		bridgeSet = append(bridgeSet, strings.ToUpper(b))
	}
	nodeSet := map[string]bool{}
	for _, n := range nodes {
		nodeSet[strings.ToUpper(n)] = true
	}
	for _, b := range bridgeSet {
		nodeSet[b] = true // a purged bridge's own node row goes too
	}

	// Removing an observer is meant to read as if it had never connected, so the
	// nodes that exist ONLY because it heard them go too. A node row is created
	// by a signature-valid advert and nothing else, and observations are kept
	// far longer than a live node can go without advertising (age-based pruning,
	// when configured, is weeks; node retention is days), so "which observers
	// evidenced this node" is answerable exactly:
	// collect the advert keys carried by the rows being deleted, and the keys
	// carried by the rows that survive. Anything in the first set and not the
	// second lost its last witness.
	//
	// Only when observers are targeted. The automatic retention sweep purges by
	// node key alone, and must not start inferring extra removals.
	orphanScan := len(obsSet) > 0
	condemned := map[string]bool{} // advert keys losing observations here
	witnessed := map[string]bool{} // advert keys still evidenced afterwards

	// ⚠ The observations table has no node column, so finding a node's rows
	// means walking every stored packet (over ten million). Doing that in ONE
	// transaction held s.mu and the single DB connection for ~2 minutes and
	// froze the site. Batches fixed the freeze but still held the lock through
	// the read AND the decode, so ingest got one operation in per batch: packets
	// backed up behind the daily sweep and behind every owner scrub, and the
	// MQTT connection dropped. So each batch now holds the connection only to
	// read, decodes with nothing held, and takes the lock only to delete.
	// Deletes commit as they go: a purge that fails part-way leaves earlier
	// batches applied, and re-running it finishes the job (it is idempotent).
	advertsOnly := len(obsSet) == 0 && len(bridgeSet) == 0
	first, last, err := s.observationIDRange()
	if err != nil {
		return res, err
	}
	for from := first - 1; from < last; {
		to := min(from+purgeBatchSize, last)
		began := time.Now()
		batch, err := s.purgeBatchRows(from, to, advertsOnly)
		if err != nil {
			return res, err
		}
		held := time.Since(began)
		delIDs := purgeMatches(batch, obsSet, bridgeSet, nodeSet, orphanScan, condemned, witnessed)
		began = time.Now()
		if err := s.deletePurged(delIDs, &res); err != nil {
			return res, err
		}
		held += time.Since(began)
		from = to
		if from >= last {
			// Packets stored while this ran: follow the table to its end.
			if _, last, err = s.observationIDRange(); err != nil {
				return res, err
			}
		}
		// Rest as long as the batch held the connection, so a purge never has
		// more than about half of it however slow the disk is.
		time.Sleep(min(held, purgeMaxRest))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	// Delete node rows for explicitly targeted nodes/bridges, along with the
	// user-authored data keyed to them (see the doc comment).
	for k := range nodeSet {
		if !cascadeUserData {
			// Retention (not a scrub): the claim outlives the node, so keep a
			// copy of what identifies it before the row goes.
			if _, err := tx.Exec(snapshotClaimSQL, k, k, k, k); err != nil {
				return res, err
			}
		}
		r, err := tx.Exec(`DELETE FROM nodes WHERE UPPER(pubkey) = ?`, k)
		if err != nil {
			return res, err
		}
		n, _ := r.RowsAffected()
		res.Nodes += n

		if !cascadeUserData {
			continue
		}
		for _, c := range []struct {
			query string
			count *int64
		}{
			{`DELETE FROM node_claims WHERE UPPER(node_pubkey) = ?`, &res.Claims},
			{`DELETE FROM node_notes WHERE UPPER(node_pubkey) = ?`, &res.Notes},
			{`DELETE FROM node_private_locations WHERE UPPER(node_pubkey) = ?`, &res.Locations},
			{`DELETE FROM location_shares WHERE UPPER(node_pubkey) = ?`, &res.Shares},
		} {
			r, err := tx.Exec(c.query, k)
			if err != nil {
				return res, err
			}
			n, _ := r.RowsAffected()
			*c.count += n
		}
	}
	// The orphan pass: node rows whose last witness just left. A user's own data
	// outranks it — someone who claimed, annotated or geolocated a node did so
	// about a real radio, and being heard by only one receiver is a coverage
	// accident rather than evidence the node was fictional. Those are held back
	// and reported instead. Anything deleted here returns on its next advert, the
	// same as a node pruned by retention.
	orphans := make([]string, 0, len(condemned))
	for k := range condemned {
		if witnessed[k] || nodeSet[k] {
			continue // still evidenced elsewhere, or already deleted above
		}
		orphans = append(orphans, k)
	}
	sort.Strings(orphans) // stable order for the log line and skipped list
	for _, k := range orphans {
		var held bool
		if err := tx.QueryRow(`SELECT EXISTS(
			SELECT 1 FROM node_claims WHERE UPPER(node_pubkey) = ?
			UNION ALL SELECT 1 FROM node_notes WHERE UPPER(node_pubkey) = ?
			UNION ALL SELECT 1 FROM node_private_locations WHERE UPPER(node_pubkey) = ?
			UNION ALL SELECT 1 FROM location_shares WHERE UPPER(node_pubkey) = ?)`,
			k, k, k, k).Scan(&held); err != nil {
			return res, err
		}
		if held {
			res.SkippedClaimed = append(res.SkippedClaimed, k)
			continue
		}
		r, err := tx.Exec(`DELETE FROM nodes WHERE UPPER(pubkey) = ?`, k)
		if err != nil {
			return res, err
		}
		n, _ := r.RowsAffected()
		res.Nodes += n
		res.OrphanNodes += n
	}

	// Delete observer rows for purged observers, along with their device
	// telemetry. The telemetry series is keyed by observer id and nothing else
	// references it, so leaving it behind orphans rows that no page can reach and
	// no sweep collects — invisible growth for every observer ever deleted.
	for o := range obsSet {
		r, err := tx.Exec(`DELETE FROM observer_telemetry WHERE observer_id = ?`, o)
		if err != nil {
			return res, err
		}
		n, _ := r.RowsAffected()
		res.Telemetry += n
		if _, err := tx.Exec(`DELETE FROM observers WHERE id = ?`, o); err != nil {
			return res, err
		}
	}

	if err := tx.Commit(); err != nil {
		return res, err
	}

	// Deleting node_claims rows invalidates the pending-claim cache that gates
	// the ingest advert verifier, so refresh it like every other writer of that
	// table does. This MUST run after Commit: loadPendingClaims issues its own
	// query, and with SetMaxOpenConns(1) it would block forever waiting on the
	// single connection an open transaction still holds.
	if res.Claims > 0 {
		if err := s.loadPendingClaims(); err != nil {
			return res, err
		}
	}
	return res, nil
}

func pathHitsBridge(path []string, bridges []string) bool {
	for _, hop := range path {
		if hop == "" {
			continue
		}
		h := strings.ToUpper(hop)
		for _, b := range bridges {
			if strings.HasPrefix(b, h) {
				return true
			}
		}
	}
	return false
}

// KnownBridges returns the set of pubkeys (uppercase) an operator has marked as
// sanctioned bridges. Detection labels these rather than hiding them: the bridge
// is real and should still be visible, it just isn't news.
func (s *Store) KnownBridges() map[string]bool {
	s.blockMu.RLock()
	defer s.blockMu.RUnlock()
	out := make(map[string]bool, len(s.knownBridges))
	for k := range s.knownBridges {
		out[k] = true
	}
	return out
}

// purgeBatchSize is how many observation ids one purge batch covers, and
// purgeDeleteChunk how many rows one delete transaction removes. purgeMaxRest
// caps the pause between batches.
const (
	purgeBatchSize   = 5000
	purgeDeleteChunk = 500
	purgeMaxRest     = 100 * time.Millisecond
)

// observationIDRange returns the lowest and highest observation ids, both 0
// when the table is empty. Two ends of the rowid b-tree, so O(1) — as separate
// subqueries because SQLite scans the whole table for MIN and MAX in one SELECT.
func (s *Store) observationIDRange() (first, last int64, err error) {
	err = s.db.QueryRow(`SELECT COALESCE((SELECT MIN(id) FROM observations), 0),
		COALESCE((SELECT MAX(id) FROM observations), 0)`).Scan(&first, &last)
	return first, last, err
}

type purgeRow struct {
	id         int64
	raw, obsID string
}

// purgeBatchRows reads the observations in (from, to]. Rows are copied out so
// the connection is free before anything is decoded.
//
// advertsOnly is for a purge that targets nodes alone: only a node's own adverts
// can match, so nine rows in ten need not leave SQLite.
func (s *Store) purgeBatchRows(from, to int64, advertsOnly bool) ([]purgeRow, error) {
	q := `SELECT id, raw_hex, COALESCE(observer_id,'') FROM observations WHERE id > ? AND id <= ?`
	args := []any{from, to}
	if advertsOnly {
		q += ` AND payload_type = ?`
		args = append(args, meshcore.PayloadAdvert.String())
	}
	rows, err := s.db.Query(q+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var batch []purgeRow
	for rows.Next() {
		var r purgeRow
		if err := rows.Scan(&r.id, &r.raw, &r.obsID); err != nil {
			return nil, err
		}
		batch = append(batch, r)
	}
	return batch, rows.Err()
}

// purgeMatches returns the ids of the rows the purge targets, and records the
// advert keys it saw for the orphan pass (condemned: on a row being deleted,
// witnessed: on a row that stays). Holds no lock.
func purgeMatches(batch []purgeRow, obsSet map[string]bool, bridgeSet []string, nodeSet map[string]bool,
	orphanScan bool, condemned, witnessed map[string]bool) []int64 {
	var delIDs []int64
	for _, r := range batch {
		byTarget := r.obsID != "" && obsSet[r.obsID]
		if byTarget && !orphanScan {
			delIDs = append(delIDs, r.id)
			continue
		}
		// Decoding an advert verifies its signature, which is most of the cost.
		// When nodes are all we are looking for, a packet that does not contain
		// a targeted key cannot be one of their adverts.
		if !orphanScan && len(bridgeSet) == 0 && !containsAnyKey(r.raw, nodeSet) {
			continue
		}
		pkt, err := meshcore.DecodeHex(r.raw)
		if err != nil || pkt == nil {
			// Undecodable: it can still be deleted for its observer, but it
			// names no node either way.
			if byTarget {
				delIDs = append(delIDs, r.id)
			}
			continue
		}
		var advertKey string
		if pkt.Advert != nil {
			advertKey = strings.ToUpper(pkt.Advert.PublicKey)
		}
		del := byTarget ||
			(advertKey != "" && nodeSet[advertKey]) ||
			(len(bridgeSet) > 0 && pathHitsBridge(pkt.RelayPath(), bridgeSet))
		if del {
			delIDs = append(delIDs, r.id)
			if advertKey != "" {
				condemned[advertKey] = true
			}
			continue
		}
		if advertKey != "" {
			witnessed[advertKey] = true
		}
	}
	return delIDs
}

// containsAnyKey reports whether a raw packet contains any of the (uppercase)
// keys. Written as DecodeHex reads it: any case, spaces allowed.
func containsAnyKey(raw string, keys map[string]bool) bool {
	raw = strings.ToUpper(strings.ReplaceAll(raw, " ", ""))
	for k := range keys {
		if strings.Contains(raw, k) {
			return true
		}
	}
	return false
}

// deletePurged deletes observations by id in short transactions.
//
// Write-only on purpose: a transaction that reads and then writes cannot upgrade
// if another connection committed in between — SQLite fails it at once with
// SQLITE_BUSY_SNAPSHOT (517), busy_timeout notwithstanding. That never happens
// inside the daemon (one connection), but does when a separate process
// (cmd/purgenodes) purges beside a live daemon.
func (s *Store) deletePurged(ids []int64, res *PurgeResult) error {
	for chunk := range slices.Chunk(ids, purgeDeleteChunk) {
		n, err := s.deleteObservations(chunk)
		if err != nil {
			return err
		}
		res.Observations += n
	}
	return nil
}

func (s *Store) deleteObservations(ids []int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var deleted int64
	for _, id := range ids {
		r, err := tx.Exec(`DELETE FROM observations WHERE id = ?`, id)
		if err != nil {
			return 0, err
		}
		n, _ := r.RowsAffected()
		deleted += n
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	s.obsCount.Add(-deleted)
	return deleted, nil
}

// snapshotClaimSQL copies a node's name, role and last-heard time onto any
// claim on it, just before the node row is deleted (args: key ×4). Missing or
// blank node values leave the claim's existing copy alone, and last_heard only
// ever moves forward.
const snapshotClaimSQL = `
	UPDATE node_claims SET
		node_name  = COALESCE((SELECT NULLIF(name,'') FROM nodes WHERE UPPER(pubkey) = ?), node_name),
		node_role  = COALESCE((SELECT NULLIF(role,'') FROM nodes WHERE UPPER(pubkey) = ?), node_role),
		last_heard = COALESCE((SELECT MAX(COALESCE(n.last_seen,''), COALESCE(n.last_heard,''), COALESCE(node_claims.last_heard,''))
		                         FROM nodes n WHERE UPPER(n.pubkey) = ?), last_heard)
	WHERE UPPER(node_pubkey) = ?`
