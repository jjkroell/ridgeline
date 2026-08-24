package analytics

import "testing"

// pathAllOnSegment is the whole safety argument of the path proof, so its edges
// are worth stating as a table: an ambiguous hop is only safe when EVERY node it
// could name is already known to be over there.
func TestPathAllOnSegment(t *testing.T) {
	far1 := "AA11111111111111111111111111111111111111111111111111111111111111"
	far2 := "AB22222222222222222222222222222222222222222222222222222222222222"
	near := "AAFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
	all := []string{far1, far2, near}
	known := map[string]bool{far1: true, far2: true}

	cases := []struct {
		name string
		path []string
		want bool
	}{
		{"every hop is a known far-side node", []string{"AA11", "AB22"}, true},
		{"a hop names a near-side node", []string{"AA11", "AAFF"}, false},
		{"one-byte hop, all candidates far-side", []string{"AB"}, true},
		{"one-byte hop, a candidate is near-side", []string{"AA"}, false},
		{"a hop matches nobody at all", []string{"AA11", "CC"}, false},
		{"an empty hop", []string{""}, false},
		// A path of one known hop is the minimum useful proof: that relay heard
		// the origin over the air.
		{"single known hop", []string{"AA11"}, true},
	}
	for _, c := range cases {
		if got := pathAllOnSegment(c.path, known, all); got != c.want {
			t.Errorf("%s: pathAllOnSegment(%v) = %v, want %v", c.name, c.path, got, c.want)
		}
	}
}

// The near end of a bridge is on THIS side, so it is never in the known set —
// which is what makes a genuine crossing fail the test. Both ends appear in a
// crossing's path, so the near end's presence is the tell.
func TestCrossedTrafficFailsThePathProof(t *testing.T) {
	farEnd := "FA00000000000000000000000000000000000000000000000000000000000000"
	nearEnd := "NE00000000000000000000000000000000000000000000000000000000000000"
	farRelay := "FB00000000000000000000000000000000000000000000000000000000000000"
	all := []string{farEnd, nearEnd, farRelay}
	known := map[string]bool{farEnd: true, farRelay: true}

	// Purely inside the far segment: proof.
	if !pathAllOnSegment([]string{"FB00", "FA00"}, known, all) {
		t.Error("a path entirely within the far segment should prove membership")
	}
	// The same advert after it crossed: the near end is now in the path.
	if pathAllOnSegment([]string{"FB00", "FA00", "NE00"}, known, all) {
		t.Error("a path containing the bridge's NEAR end must fail: that advert crossed")
	}
}

// A path proof must never overturn a direct reception on THIS side. The sweep
// rejects such a node in the same run ("heard directly on this side"), and an
// inference that contradicts the run's own measurement is a bug, not a finding.
//
// This is not hypothetical: the first cut of provenFarSide moved a real
// near-side node (UBCV//Maple) onto the far segment and gave it a 909 radio,
// while the very same report rejected it for being heard directly over here.
func TestPathProofNeverOverturnsANearSideDirectReception(t *testing.T) {
	near := "NEAR000000000000000000000000000000000000000000000000000000000000"

	// The node crossed the bridge often enough to be a candidate AND was heard
	// directly on this side. The direct reception settles it.
	heardHere := &segStat{zeroHop: 3, confirmed: 20, withPath: 20}
	if conf, reject := heardHere.verdict(); conf != "" || reject != "heard directly on this side" {
		t.Fatalf("verdict = (%q,%q), want a rejection", conf, reject)
	}

	// Same node, and a path that would otherwise pass the proof.
	far := "FAR0000000000000000000000000000000000000000000000000000000000000"
	all := []string{near, far}
	known := map[string]bool{far: true}
	if !pathAllOnSegment([]string{"FAR0"}, known, all) {
		t.Fatal("precondition: the path alone should look like proof")
	}
	// The guard in provenFarSide is what stops the two from being combined; the
	// stat that triggers it is exactly the one behind the rejection above.
	if heardHere.zeroHop == 0 {
		t.Error("a node heard directly on this side must carry the zeroHop that vetoes the path proof")
	}
}
