package analytics

import "testing"

func TestTxAggCopies(t *testing.T) {
	cases := []struct {
		name    string
		tx      txAgg
		copies  int
		relayed bool
	}{
		{"flood heard direct only", txAgg{flood: true}, 1, false},
		{"flood, 3 distinct relays across copies", txAgg{flood: true, floodHops: map[string]bool{"AB": true, "CD": true, "EF": true}}, 4, true},
		{"routed zero-hop", txAgg{}, 1, false},
		{"routed heard with 2 hops left", txAgg{maxHops: 2}, 3, true},
	}
	for _, c := range cases {
		if got := c.tx.copies(); got != c.copies {
			t.Errorf("%s: copies=%d want %d", c.name, got, c.copies)
		}
		if got := c.tx.relayed(); got != c.relayed {
			t.Errorf("%s: relayed=%v want %v", c.name, got, c.relayed)
		}
	}
}

func TestCongestionTier(t *testing.T) {
	for pct, want := range map[float64]string{0.5: "Quiet", 3: "Normal", 8: "Busy", 15: "Congested"} {
		if got := congestionTier(pct); got != want {
			t.Errorf("%v%%: %s want %s", pct, got, want)
		}
	}
}
