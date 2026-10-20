package ingest

import (
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/jjkroell/ridgeline/internal/store"
)

// Bounds on an untrusted /neighbors payload. openHop caps a table at 32 rows by
// default and firmware at what fits 10 KB; these leave headroom for both while
// keeping a hostile publisher from storing anything large.
const (
	maxNeighborPayload = 64 << 10
	maxNeighborRows    = 64
	maxScopesLen       = 200
	maxHeardSecsAgo    = 30 * 86400
)

// neighborsEnvelope is the JSON published on meshcore/{IATA}/{PUBKEY}/neighbors.
type neighborsEnvelope struct {
	Timestamp string `json:"timestamp"`
	Origin    string `json:"origin"`
	OriginID  string `json:"origin_id"`
	Self      struct {
		Scopes       string `json:"scopes"`
		DefaultScope string `json:"default_scope"`
	} `json:"self"`
	Neighbors []struct {
		Pubkey       string   `json:"pubkey"`
		SNR          *float64 `json:"snr"`
		HeardSecsAgo *float64 `json:"heard_secs_ago"`
		Scopes       string   `json:"scopes"`
		Status       string   `json:"status"`
	} `json:"neighbors"`
}

func isTopicNeighbors(topic string) bool {
	return strings.HasSuffix(topic, "/neighbors") || strings.HasSuffix(topic, "/neighbours")
}

// handleNeighbors stores a repeater's published neighbour table.
//
// The reporter is the public key in the TOPIC, never the payload's origin_id:
// the broker's ACL binds an observer to its own topic subtree, so the topic key
// is the authenticated identity and the payload is only a claim.
//
// The same gates as packets apply, for the same reasons — a blocked, stood-down
// or wrong-preset observer is describing a network that is not this one — plus
// one of its own: a retained copy is a replay of an old table, and stamping it
// with today's receive time would present stale neighbours as current.
func (in *Ingestor) handleNeighbors(msg mqtt.Message) {
	if msg.Retained() || len(msg.Payload()) > maxNeighborPayload {
		return
	}
	observerKey, _ := topicMeta(msg.Topic())
	reporter := strings.ToUpper(observerKey)
	if !isFullKey(reporter) {
		in.log.Debug("neighbors topic without a full public key", "topic", msg.Topic())
		return
	}
	if in.store.IsObserverBlocked(observerKey) || in.store.ObserverOnStandby(observerKey) ||
		in.store.ObserverRadioQuarantined(observerKey) ||
		(in.store.AllowedRadioCount() > 0 && !in.store.ObserverRadioConfirmed(observerKey)) {
		in.log.Debug("ignored neighbors from an observer not currently trusted", "observer", observerKey)
		return
	}
	var env neighborsEnvelope
	if err := json.Unmarshal(msg.Payload(), &env); err != nil {
		in.log.Debug("bad neighbors envelope", "topic", msg.Topic(), "err", err)
		return
	}
	r := store.NeighborReport{
		Reporter:     reporter,
		ObserverID:   observerKey,
		ReportedAt:   time.Now().UTC().Format(time.RFC3339),
		SelfScopes:   clip(env.Self.Scopes, maxScopesLen),
		DefaultScope: clip(env.Self.DefaultScope, maxScopesLen),
		Neighbors:    []store.NeighborEntry{},
	}
	seen := map[string]bool{}
	for _, n := range env.Neighbors {
		if len(r.Neighbors) >= maxNeighborRows {
			break
		}
		pk := strings.ToUpper(n.Pubkey)
		if !isFullKey(pk) || pk == reporter || seen[pk] {
			continue
		}
		seen[pk] = true
		e := store.NeighborEntry{
			PublicKey: pk,
			Scopes:    clip(n.Scopes, maxScopesLen),
			Status:    clip(n.Status, 20),
		}
		if n.SNR != nil && !math.IsNaN(*n.SNR) && *n.SNR >= -40 && *n.SNR <= 40 {
			v := math.Round(*n.SNR*10) / 10
			e.SNR = &v
		}
		// null is the firmware's "unknown" (heard before its clock was set), not
		// zero: keep the row, mark the age unknown. An out-of-range age is garbage.
		switch {
		case n.HeardSecsAgo == nil:
			e.HeardSecsAgo = -1
		case *n.HeardSecsAgo >= 0 && *n.HeardSecsAgo <= maxHeardSecsAgo:
			e.HeardSecsAgo = int64(*n.HeardSecsAgo)
		default:
			continue
		}
		r.Neighbors = append(r.Neighbors, e)
	}
	if err := in.store.SaveNeighborReport(r); err != nil {
		in.log.Error("store neighbor report failed", "reporter", reporter, "err", err)
		return
	}
	in.log.Info("neighbor report stored", "reporter", reporter, "name", env.Origin, "neighbors", len(r.Neighbors))
}

// isFullKey reports whether s is a 32-byte public key in hex.
func isFullKey(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}
