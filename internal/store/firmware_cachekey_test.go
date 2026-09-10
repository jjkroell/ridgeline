package store

import "testing"

// Option order carries no meaning to the compiler, so two requests that differ
// only in ordering must land on the same cache key — otherwise identical
// firmware is compiled twice and the cache quietly never hits.
func TestCanonicalFlagsIsOrderIndependent(t *testing.T) {
	a := CanonicalFlags([]string{"-D B=1", "-D A=1"})
	b := CanonicalFlags([]string{"-D A=1", "-D B=1"})
	if a != b {
		t.Fatalf("canonical forms differ: %q vs %q", a, b)
	}
	if FirmwareCacheKey("t", "e", a) != FirmwareCacheKey("t", "e", b) {
		t.Error("cache keys differ for equivalent flag sets")
	}
}

func TestCanonicalFlagsDropsBlanks(t *testing.T) {
	if got := CanonicalFlags([]string{"", "  ", "-D A=1"}); got != "-D A=1" {
		t.Errorf("CanonicalFlags = %q, want %q", got, "-D A=1")
	}
}

// Tag, env and flags must all participate: a key that ignores any of them would
// serve one board's firmware to another.
func TestCacheKeyDistinguishesEveryInput(t *testing.T) {
	base := FirmwareCacheKey("repeater-v1.17.1", "RAK_4631_repeater", "-D A=1")
	for _, k := range []string{
		FirmwareCacheKey("repeater-v1.18.0", "RAK_4631_repeater", "-D A=1"),
		FirmwareCacheKey("repeater-v1.17.1", "Heltec_v3_repeater", "-D A=1"),
		FirmwareCacheKey("repeater-v1.17.1", "RAK_4631_repeater", ""),
	} {
		if k == base {
			t.Error("cache key collides across differing requests")
		}
	}
}
