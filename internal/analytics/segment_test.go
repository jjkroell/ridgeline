package analytics

import (
	"testing"

	"github.com/jjkroell/ridgeline/internal/store"
)

// The rule under test, stated as a table so each clause is visible. It calls
// segStat.verdict directly rather than restating the conditions: a test that
// re-implements the rule agrees with itself no matter what the code does.
//
// Two kinds of evidence are in play. A NEAR-side receiver hearing a node
// directly refutes membership; a FAR-side receiver hearing it directly proves
// it. Everything else is inferred from traffic crossing the bridge.
func TestSegmentVerdict(t *testing.T) {
	cases := []struct {
		name string
		s    segStat
		want string // "" = not a candidate, else confidence or rejection reason
		why  string
	}{
		{"far-side node, all traffic crosses", segStat{withPath: 60, confirmed: 60}, "confirmed",
			"the shape every real far-side node had on the live mesh"},
		{"heard directly even once", segStat{withPath: 60, confirmed: 60, zeroHop: 1},
			"heard directly on this side",
			"a receiver on this side cannot hear the far side, so one direct sighting disqualifies"},
		{"routed through occasionally", segStat{withPath: 220, confirmed: 5},
			"only some traffic crosses the bridge",
			"2% is a packet taking an odd route, not a node living over there"},
		{"just under the share bar", segStat{withPath: 100, confirmed: 89}, "only some traffic crosses the bridge", ""},
		{"just over the share bar", segStat{withPath: 100, confirmed: 90}, "confirmed", ""},
		{"too few sightings to judge", segStat{withPath: 3, confirmed: 3}, "too few sightings to judge",
			"100% of three proves nothing"},
		{"probable only (1-byte path)", segStat{withPath: 40, probable: 40}, "probable",
			"a narrow originator must not be invisible - companions skew narrow"},
		{"no evidence either way", segStat{withPath: 40}, "",
			"never crossed and never heard over there: not a candidate, and not a rejection to report"},

		// Far-side receivers.
		{"heard directly on the far segment", segStat{zeroHopFar: 3}, "observed",
			"a receiver on that frequency settles what the crossing rule only estimates"},
		{"heard over there, never crossed while watching", segStat{zeroHopFar: 1}, "observed",
			"direct reception stands alone - it does not need a crossing to corroborate it"},
		{"heard over there AND crossing", segStat{zeroHopFar: 5, withPath: 60, confirmed: 60}, "observed",
			"measurement outranks inference when both agree"},
		{"heard over there but too few crossings", segStat{zeroHopFar: 5, withPath: 2, confirmed: 2}, "observed",
			"the sightings bar guards an INFERENCE; it must not veto a measurement"},
		{"heard on both sides", segStat{zeroHopFar: 5, zeroHop: 5, withPath: 60, confirmed: 60},
			"heard directly from both sides",
			"a contradiction is surfaced, never silently resolved in favour of one side"},

		// The regression this whole change exists to prevent.
		{"far-side node once a far-side receiver exists",
			segStat{zeroHopFar: 4, withPath: 60, confirmed: 54}, "observed",
			"before observer segmentation this node's own far-side receiver disqualified it"},
	}
	for _, c := range cases {
		conf, reject := c.s.verdict()
		got := conf
		if got == "" {
			got = reject
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q (%s)", c.name, got, c.want, c.why)
		}
	}
}

// Confidence reflects HOW membership was established, which the console and the
// node page show so an inference is never mistaken for a measurement.
func TestSegmentConfidence(t *testing.T) {
	for _, c := range []struct {
		s    segStat
		want string
	}{
		{segStat{withPath: 10, confirmed: 10}, "confirmed"},
		{segStat{withPath: 100, confirmed: 1, probable: 99}, "confirmed"}, // any wide proof wins
		{segStat{withPath: 10, probable: 10}, "probable"},
		{segStat{zeroHopFar: 1}, "observed"}, // direct reception outranks both
	} {
		if got, _ := c.s.verdict(); got != c.want {
			t.Errorf("%+v: got %s, want %s", c.s, got, c.want)
		}
	}
}

// A bridge with no peer recorded cannot define a segment — a link needs two
// ends — so the sweep must do nothing rather than guess.
func TestDetectSegmentsNoLinksIsNoOp(t *testing.T) {
	rep, err := DetectSegments(nil, nil, nil, nil, "2026-08-15T23:30:00Z", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Members) != 0 || rep.Scanned != 0 {
		t.Errorf("expected an empty report, got %+v", rep)
	}
}

// The far side's declared radio must never be confused with a node's own
// inherited value; ApplySegments/annotate blank the latter. This pins the
// store-level contract the API depends on.
func TestSegmentMemberShape(t *testing.T) {
	m := store.SegmentMember{NodeKey: "abc", BridgeKey: "def", Confidence: "confirmed"}
	switch m.Confidence {
	case "observed", "confirmed", "probable":
	default:
		t.Errorf("confidence must be one of the three documented values, got %q", m.Confidence)
	}
}

// Sorting receivers onto the right side of the bridge is the input everything
// else depends on: place a near-side receiver on the far segment and its ordinary
// direct receptions start "proving" membership for half the mesh.
func TestFarObserverSets(t *testing.T) {
	link := store.BridgeLink{
		Key:       "AAAA",
		Near:      "BBBB",             // this side of the wire
		Far:       "AAAA",             // the end on the 909 segment
		PeerRadio: "909.000,62.5,8,5", // as an operator types it
	}
	observers := []store.Observer{
		{ID: "far-909", Radio: "909.0,62.5,8,5"},           // as the receiver reports it
		{ID: "near-a", Radio: "910.4249877,62.5,7,5"},      // synthesised centre
		{ID: "near-b", Radio: "910.425,62.5,7,5"},          // same channel, rounded
		{ID: "near-c", Radio: "910.425,62.5,7,8"},          // same channel, other coding rate
		{ID: "unknown", Radio: ""},                         // never reported one
		{ID: "far-freq-wrong-sf", Radio: "909.0,62.5,7,5"}, // right channel, cannot demodulate
	}

	sets, n := farObserverSets([]store.BridgeLink{link}, observers)
	far := sets[link.Key]

	if !far["far-909"] {
		t.Error("the 909 receiver must be recognised despite 909.000 vs 909.0")
	}
	for _, id := range []string{"near-a", "near-b", "near-c", "unknown", "far-freq-wrong-sf"} {
		if far[id] {
			t.Errorf("%s must stay near-side", id)
		}
	}
	if n != 1 {
		t.Errorf("FarObservers = %d, want 1", n)
	}

	// No declared far side: nobody can be placed, and the detector falls back to
	// the traffic rule exactly as it behaved before receivers were sorted at all.
	if sets, n := farObserverSets([]store.BridgeLink{{Key: "AAAA", Near: "BBBB", Far: "AAAA"}}, observers); n != 0 || len(sets["AAAA"]) != 0 {
		t.Error("a bridge with no declared far radio must classify nobody")
	}
}

// The two ends of a bridge appear in a great many paths that never crossed
// anything, so their PRESENCE proves nothing — only the ORDER they appear in
// does. A node living on the far segment must send its traffic across the wire
// to reach a receiver here, so its path enters at the far end and leaves at the
// near one.
//
// This is the test that would have caught the ends being labelled backwards.
// Reading the order against the wrong end does not error: it turns every real
// member into a reverse crossing and reports an empty far side, which looks
// exactly like a bridge nobody lives behind.
func TestCrossingDirection(t *testing.T) {
	// Two ends far enough apart that no prefix of one matches the other.
	const farEnd = "A1B2C3D4E5F60718"
	const nearEnd = "B2C3D4E5F6071829"
	link := store.BridgeLink{Key: farEnd, Near: nearEnd, Far: farEnd, PeerRadio: "909.000,62.5,8,5"}

	cases := []struct {
		name       string
		path       []string
		byteUnique bool
		want       string
	}{
		{
			// The shape the live mesh produces: several relays on the far
			// segment, the wire, then relays on ours before a receiver here.
			name: "far-side origin reaching a near-side receiver",
			path: []string{"9D8126", "C20CF0", "A1B2C3", "B2C3D4", "56E6C2"},
			want: crossConfirmed,
		},
		{
			name: "one of ours on its way out is not a crossing",
			path: []string{"5BF2E4", "B2C3D4", "A1B2C3", "040D5F"},
			want: crossReverse,
		},
		{
			name: "adjacent ends, nothing either side",
			path: []string{"A1B2C3", "B2C3D4"},
			want: crossConfirmed,
		},
		{
			name: "only one end present proves nothing",
			path: []string{"9D8126", "A1B2C3", "56E6C2"},
			want: crossNone,
		},
		{
			name: "neither end present",
			path: []string{"9D8126", "C20CF0", "56E6C2"},
			want: crossNone,
		},
		{
			name:       "one-byte hops are admitted when the far byte is unique",
			path:       []string{"9D", "A1", "B2", "56"},
			byteUnique: true,
			want:       crossProbable,
		},
		{
			// A 1-byte match only says no KNOWN node shares that byte; an
			// unknown one silently would. Without uniqueness there is no claim.
			name:       "one-byte hops are refused when the far byte is shared",
			path:       []string{"9D", "A1", "B2", "56"},
			byteUnique: false,
			want:       crossNone,
		},
		{
			name:       "one-byte hops in the outbound order are not a crossing",
			path:       []string{"B2", "A1"},
			byteUnique: true,
			want:       crossNone,
		},
		{
			// Width is the ORIGINATOR's setting and every relay appends at it, so
			// a path is all one width; a wide match must never be satisfied by a
			// narrow hop that merely shares a byte.
			name:       "a wide crossing is not read out of narrow hops",
			path:       []string{"A1", "B2"},
			byteUnique: false,
			want:       crossNone,
		},
		{
			name: "empty path",
			path: nil,
			want: crossNone,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := crossingKind(c.path, link, c.byteUnique); got != c.want {
				t.Errorf("crossingKind(%v) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}

// Swapping the ends is survivable only because it is visible. A bridge recorded
// end-for-end still counts crossings, all of them backwards, so a lopsided
// reverse tally is the signal — and the only one, since the wrong labelling
// produces no error and no members.
func TestEndsLookReversed(t *testing.T) {
	cases := []struct {
		name             string
		forward, reverse int
		want             bool
	}{
		{"a healthy bridge crosses inbound", 800, 5, false},
		{"ends recorded the wrong way round", 3, 240, true},
		{"a quiet new bridge is never accused", 0, 8, false},
		{"exactly at the floor, wholly one-way", 0, segMinReverse, true},
		{"reverse-heavy but not lopsided enough", 10, 30, false},
		{"nothing seen at all", 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := endsLookReversed(c.forward, c.reverse); got != c.want {
				t.Errorf("endsLookReversed(%d, %d) = %v, want %v", c.forward, c.reverse, got, c.want)
			}
		})
	}
}
