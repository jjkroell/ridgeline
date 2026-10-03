package store

import "testing"

// The real keys from the Mt Cokley RF bridge and the nodes that collide with
// them on a 1-byte path hop. Using the live values on purpose: the whole reason
// this block matches a PAIR instead of one end is that these prefixes overlap,
// and a synthetic key would not demonstrate it.
const (
	cokley425  = "22563EB8BD21872846DE246B1043BBC3B0A56CC26645A64B57D6D2394330954A" // A, on 910.425
	mtCokley   = "777C746B6F1B7A929DD7E344DAA0A9B3B88AD4D2D9EED698E6B3A29C447706E4" // B, on 909.0
	mtBrenton  = "2235D82AE3290011223344556677889900AABBCCDDEEFF00112233445566778A" // collides with A at 1 byte (22)
	nbPager    = "7733CA34D6310011223344556677889900AABBCCDDEEFF00112233445566778B" // collides with B at 1 byte (77)
	someOrigin = "FE01020304050607080900AABBCCDDEEFF00112233445566778899AABBCCDDEE"
)

// TestBlockedLinkMatchesDirectedPair is the core of the rule: traffic that
// crossed the bridge INTO this mesh is refused, and nothing else is.
func TestBlockedLinkMatchesDirectedPair(t *testing.T) {
	st := testStore(t)
	if err := st.AddBlockPeer(BlockLink, cokley425, "Cokley 425 -> Mt Cokley", "909 cutover: done with 910.425", mtCokley); err != nil {
		t.Fatalf("add link: %v", err)
	}

	// A path hop's width is the ORIGINATOR's setting, so the same crossing
	// arrives as 1-, 2-, or 3-byte hops. All three must match.
	for _, tc := range []struct {
		name string
		path []string
	}{
		{"1-byte hops", []string{"22", "77"}},
		{"2-byte hops", []string{"2256", "777C"}},
		{"3-byte hops", []string{"22563E", "777C74"}},
		{"pair mid-path", []string{"A1", "22", "77", "B2"}},
		{"pair at the end", []string{"A1", "B2", "2256", "777C"}},
	} {
		if !st.ShouldDrop(advertPkt(someOrigin, tc.path...), "obs-a") {
			t.Errorf("%s: crossing %v was not dropped", tc.name, tc.path)
		}
	}
}

// TestBlockedLinkIgnoresEverythingElse pins the negatives. Each of these would
// be dropped by a looser rule, and each is traffic worth keeping.
func TestBlockedLinkIgnoresEverythingElse(t *testing.T) {
	st := testStore(t)
	if err := st.AddBlockPeer(BlockLink, cokley425, "link", "909 cutover", mtCokley); err != nil {
		t.Fatalf("add link: %v", err)
	}

	for _, tc := range []struct {
		name, why string
		path      []string
	}{
		{"reverse direction", "this mesh's own traffic leaking OUT to 910.425, not foreign traffic coming in",
			[]string{"777C", "2256"}},
		{"ends present but not adjacent", "a packet that touched both at different points never crossed the link",
			[]string{"2256", "A1", "777C"}},
		{"near end alone", "Cokley 425 relaying within its own segment is not a crossing",
			[]string{"2256", "A1"}},
		{"far end alone", "Mt Cokley is a working 909 repeater; its own relaying must survive",
			[]string{"777C", "A1"}},
		{"1-byte collision pair", "MT BRENTON (22…) relaying to N.B. (77…) is NOT the bridge — this is the case a single-key prefix block gets wrong",
			[]string{"2235D8", "7733CA"}},
		{"empty path", "a zero-hop reception has no link to cross", nil},
	} {
		if st.ShouldDrop(advertPkt(someOrigin, tc.path...), "obs-a") {
			t.Errorf("%s: wrongly dropped %v — %s", tc.name, tc.path, tc.why)
		}
	}

	// Neither end may be hidden or origin-blocked by a link. Mt Cokley in
	// particular is a live repeater on this mesh.
	for _, pk := range []string{cokley425, mtCokley} {
		if st.IsNodeBlocked(pk) {
			t.Errorf("link block must not mark %s… as a blocked node", pk[:8])
		}
		if st.ShouldDrop(advertPkt(pk), "obs-a") {
			t.Errorf("link block must not drop %s…'s own advert", pk[:8])
		}
	}
}

// TestBlockedLinkNeedsBothEnds guards the half-formed case: a link row with no
// peer must not degrade into a single-key prefix block, which is exactly the
// over-matching behaviour the pair rule exists to avoid.
func TestBlockedLinkNeedsBothEnds(t *testing.T) {
	st := testStore(t)
	if err := st.AddBlock(BlockLink, cokley425, "half", "no peer"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if st.ShouldDrop(advertPkt(someOrigin, "2256", "777C"), "obs-a") {
		t.Error("a link with no far end must be inert, not a loose single-key block")
	}
	if st.ShouldDrop(advertPkt(someOrigin, "22"), "obs-a") {
		t.Error("a link with no far end must not block on its near end alone")
	}
}

// TestBlockedLinkRemovable covers the reverse: the 910.425 side could come back,
// and nothing about this should need a redeploy to undo.
func TestBlockedLinkRemovable(t *testing.T) {
	st := testStore(t)
	if err := st.AddBlockPeer(BlockLink, cokley425, "link", "cutover", mtCokley); err != nil {
		t.Fatalf("add: %v", err)
	}
	if !st.ShouldDrop(advertPkt(someOrigin, "2256", "777C"), "obs-a") {
		t.Fatal("precondition: crossing should drop while the link is blocked")
	}
	// Case-insensitive, like every other pubkey-keyed block.
	if err := st.RemoveBlock(BlockLink, "22563eb8bd21872846de246b1043bbc3b0a56cc26645a64b57d6d2394330954a"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if st.ShouldDrop(advertPkt(someOrigin, "2256", "777C"), "obs-a") {
		t.Error("still dropping crossings after the link was removed")
	}
}

// TestBlockedLinkCoexistsWithKnownBridge covers the migration shape: the same
// node is currently marked kind='known' (a sanctioned bridge) and is about to
// also carry a kind='link' block. The two keys are independent rows, so the
// link must bite even while the bridge is still marked known.
func TestBlockedLinkCoexistsWithKnownBridge(t *testing.T) {
	st := testStore(t)
	if err := st.AddBlockPeer(BlockKnown, cokley425, "KOD - Cokley 909", "operator's own bridge", mtCokley); err != nil {
		t.Fatalf("mark known: %v", err)
	}
	if st.ShouldDrop(advertPkt(someOrigin, "2256", "777C"), "obs-a") {
		t.Fatal("precondition: a known bridge alone must not drop anything")
	}
	if err := st.AddBlockPeer(BlockLink, cokley425, "Cokley 425 -> Mt Cokley", "909 cutover", mtCokley); err != nil {
		t.Fatalf("add link: %v", err)
	}
	if !st.ShouldDrop(advertPkt(someOrigin, "2256", "777C"), "obs-a") {
		t.Error("link block did not bite while the same node was still marked known")
	}
	if k := st.KnownBridges(); !k[cokley425] {
		t.Error("adding a link must not disturb the sanctioned-bridge registry")
	}
}
