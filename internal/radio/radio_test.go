package radio

import "testing"

// The case this package exists for: one channel, two spellings, arriving from
// real observers on the live mesh.
func TestOneChannelTwoSpellings(t *testing.T) {
	a := "910.4249877,62.5,7,5" // a radio reporting its synthesised centre
	b := "910.425,62.5,7,5"     // the same channel, rounded

	if !SameSegmentString(a, b) {
		t.Error("910.4249877 and 910.425 must be one segment")
	}
	if Normalize(a) != b {
		t.Errorf("Normalize(%q) = %q, want %q", a, Normalize(a), b)
	}
	if Normalize(b) != b {
		t.Errorf("Normalize is not idempotent: %q -> %q", b, Normalize(b))
	}
}

// An operator declaring a bridge's far side types a different number of zeros
// than the receiver on that side reports.
func TestDeclaredAndReportedAgree(t *testing.T) {
	if !SameSegmentString("909.000,62.5,8,5", "909.0,62.5,8,5") {
		t.Error("909.000 (declared) and 909.0 (reported) must be one segment")
	}
}

func TestSegmentBoundaries(t *testing.T) {
	base := "910.425,62.5,7,5"
	cases := []struct {
		other string
		same  bool
		why   string
	}{
		{"910.425,62.5,7,8", true, "coding rate is carried in the LoRa header, so it does not split a network"},
		{"909.0,62.5,7,5", false, "a different channel is a different network"},
		{"910.425,250,7,5", false, "a different bandwidth cannot be demodulated"},
		{"910.425,62.5,8,5", false, "a different spreading factor cannot be demodulated"},
		{"910.4249877,62.5,7,5", true, "synthesiser rounding is not a difference"},
		{"910.428,62.5,7,5", true, "3 kHz is inside the tolerance"},
		{"910.440,62.5,7,5", false, "15 kHz away is a different channel, not a rounding artefact"},
		{"", false, "an unknown config must never match"},
		{"910.425", false, "a frequency alone is not enough to claim a match"},
		{"garbage", false, "unparseable is unknown"},
	}
	for _, c := range cases {
		if got := SameSegmentString(base, c.other); got != c.same {
			t.Errorf("SameSegmentString(%q, %q) = %v, want %v (%s)", base, c.other, got, c.same, c.why)
		}
	}
	// Unknown must not match unknown either, or every observer with no reported
	// radio would land in one fictitious segment together.
	if SameSegmentString("", "") {
		t.Error("two unknown configs must not match each other")
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"910.4249877,62.5,7,5": "910.425,62.5,7,5",
		"909.0,62.5,8,5":       "909.0,62.5,8,5",
		"909,62.5,8,5":         "909.0,62.5,8,5", // always reads as a frequency
		" 910.425 , 62.5 ,7,5": "910.425,62.5,7,5",
		"910.4254999,62.5,7,5": "910.425,62.5,7,5",
		"910.4255,62.5,7,5":    "910.426,62.5,7,5", // kHz precision, not coarser
		"":                     "",
		"garbage,62.5":         "garbage,62.5", // returned verbatim, not mangled
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseLeavesMissingUnset(t *testing.T) {
	p := Parse("910.425")
	if !p.HasFreq || p.HasBW || p.HasSF || p.HasCR {
		t.Errorf("Parse(%q) = %+v, want only the frequency set", "910.425", p)
	}
	// A zero must not read as "known to be zero" — two unknowns would compare equal.
	if q := Parse("garbage,62.5,7,5"); q.HasFreq {
		t.Errorf("unparseable frequency reported as known: %+v", q)
	}
}
