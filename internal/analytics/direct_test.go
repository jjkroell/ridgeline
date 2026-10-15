package analytics

import (
	"strings"
	"testing"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

const discoverKey = "169444F317F877B5FBC5C8D95CA4DA48F198AA478113B7DF3C1D136538DFB06B"

// discoverReply builds a flood CONTROL node-discover reply (role repeater,
// SNR byte, tag) carrying key, with the given header path bytes.
func discoverReply(path, key string) string {
	return "2D" + path + "92" + "F4" + "11223344" + key
}

func TestHeardDirectly(t *testing.T) {
	adv, err := meshcore.DecodeHex(advertFixture)
	if err != nil || adv.Advert == nil {
		t.Fatalf("decode advert: %v", err)
	}
	cases := []struct {
		name, raw, observer, want string
	}{
		{"zero-hop discover reply", discoverReply("00", discoverKey), "obs-A", discoverKey},
		{"lower-case key is normalised", discoverReply("00", strings.ToLower(discoverKey)), "obs-A", discoverKey},
		{"relayed reply proves nothing", discoverReply("01AB", discoverKey), "obs-A", ""},
		{"prefix-only reply is ambiguous", discoverReply("00", discoverKey[:16]), "obs-A", ""},
		{"odd-length key is rejected", discoverReply("00", discoverKey+"0102"), "obs-A", ""},
		{"own reply is not a neighbour", discoverReply("00", discoverKey), strings.ToLower(discoverKey), ""},
		{"no observer", discoverReply("00", discoverKey), "", ""},
		{"discover request names no one", "2D00" + "80" + "04" + "11223344", "obs-A", ""},
		{"zero-hop advert", advertFixture, "obs-A", strings.ToUpper(adv.Advert.PublicKey)},
		{"own advert", advertFixture, adv.Advert.PublicKey, ""},
	}
	for _, c := range cases {
		pkt, err := meshcore.DecodeHex(c.raw)
		if err != nil || pkt == nil || !pkt.Valid {
			t.Fatalf("%s: decode: %v %+v", c.name, err, pkt)
		}
		if got := heardDirectly(pkt, c.observer); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
