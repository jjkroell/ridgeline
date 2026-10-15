package analytics

import (
	"strings"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

// heardDirectly returns the public key of the node that observerID heard over
// direct RF in pkt, or "" when the packet proves no such link.
//
// Two packet kinds prove one, both because they are sent zero-hop and name
// their sender by full public key:
//   - an advert heard with an empty relay path;
//   - a node-discover reply (CONTROL 0x90). Repeaters always send it zero-hop
//     (sendZeroHop in simple_repeater), so whoever hears it heard the
//     repeater. Only full 32-byte keys count; a prefix-only reply could name
//     several nodes. The SNR byte inside the reply is how well the repeater
//     heard the REQUEST, whose sender is unknown, so it is ignored here — the
//     observation's own SNR/RSSI is the link's.
//
// An observer's id is its node's public key, and some observers publish the
// packets they transmit themselves (their own discover replies show up as
// "heard at zero hops"), so a node is never its own observer's neighbour.
func heardDirectly(pkt *meshcore.Packet, observerID string) string {
	if observerID == "" || len(pkt.RelayPath()) != 0 {
		return ""
	}
	var key string
	switch {
	case pkt.Advert != nil:
		key = pkt.Advert.PublicKey
	case pkt.Control != nil && pkt.Control.SubType == meshcore.ControlNodeDiscoverResp:
		key = pkt.Control.PublicKey
		if len(key) != 64 {
			return ""
		}
	}
	key = strings.ToUpper(key)
	if key == "" || strings.EqualFold(key, observerID) {
		return ""
	}
	return key
}
