package analytics

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// A real advert whose Ed25519 signature verifies (from the decoder fixtures).
const signedAdvert = "10B76000008A654F144C4A43D07F024E8E0A59120F9A3C2E825453F88F861C48C2F2BA245CD672C56C5BE3F52CB4470337904147543724983C2978DCCE234CE41898674714704C6A642E674B47350417E441B17CF5CC4BF444792266EDC3F4FE7970F4DF977CA9344CE16A45FEA7FCD25A85E9653FD2F1DB63666B0A792290A37F398341E70B61099279FFED021081B5F8F09F8D924368657272792048696C6C20F09F8D92"

// TestDetectInjectionRejectsUnsignedAdverts covers the signature gate: an advert
// whose Ed25519 signature does not verify carries an untrustworthy public key, so
// it invents an origin that never existed. Those phantoms land in the injector
// rule — a handful of one-off keys seen by a single observer is exactly its
// "sole source of many origins" signature — so they must never reach scoring.
func TestDetectInjectionRejectsUnsignedAdverts(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "detect.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	// Sanity: the fixture really is a signature-valid advert.
	pkt, err := meshcore.DecodeHex(signedAdvert)
	if err != nil || pkt == nil || pkt.Advert == nil {
		t.Fatalf("fixture did not decode as an advert: %v", err)
	}
	if !pkt.Advert.SignatureValid {
		t.Fatal("fixture must have a valid signature for this test to mean anything")
	}

	// Build several DISTINCT phantom origins: mutate bytes inside the advert's
	// public key so each decodes as a different node whose signature no longer
	// verifies. One observer reporting only these is precisely the injector
	// rule's "sole source of many origins" signature.
	now := time.Now().UTC()
	upper := strings.ToUpper(signedAdvert)
	keyAt := strings.Index(upper, pkt.Advert.PublicKey)
	if keyAt < 0 {
		t.Fatal("could not locate the public key inside the raw advert")
	}
	phantoms := 0
	for i := 0; i < 4; i++ {
		b := []byte(upper)
		// Distinct mutation per phantom, inside the key only.
		for j := 0; j < 4; j++ {
			pos := keyAt + i*4 + j
			if b[pos] == 'A' {
				b[pos] = 'B'
			} else {
				b[pos] = 'A'
			}
		}
		raw := string(b)
		bp, err := meshcore.DecodeHex(raw)
		if err != nil || bp == nil || bp.Advert == nil {
			continue
		}
		if bp.Advert.SignatureValid || bp.Advert.PublicKey == pkt.Advert.PublicKey {
			continue // not a usable phantom
		}
		phantoms++
		for r := 0; r < 3; r++ {
			if err := st.Record(store.Observation{
				Packet: bp, RawHex: raw, ObserverID: "obs-solo",
				ReceivedAt: now.Add(-time.Duration(i*3+r) * time.Minute),
			}); err != nil {
				t.Fatalf("record: %v", err)
			}
		}
	}
	if phantoms < minExclusiveNodes {
		t.Skipf("only produced %d distinct phantom origins; need %d to trip the injector rule",
			phantoms, minExclusiveNodes)
	}

	cut := now.Add(-6 * time.Hour).Format(time.RFC3339Nano)
	rep, err := DetectInjection(st, nil, cut, 0)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if rep.AdvertsRejected != phantoms*3 {
		t.Errorf("AdvertsRejected = %d, want %d", rep.AdvertsRejected, phantoms*3)
	}
	if len(rep.Injectors) != 0 {
		t.Errorf("unsigned adverts produced %d injector candidate(s); the phantom "+
			"origin should never have been scored", len(rep.Injectors))
	}
}

// TestDetectInjectionUsesAllPayloadTypes covers the path-evidence change: a
// packet's route is in the clear whatever its payload, so every payload type
// contributes. Before this, the scan skipped anything that wasn't an advert,
// which made a companion that never adverts contribute nothing at all — even
// though its messages crossed the bridge with a full path attached.
func TestDetectInjectionUsesAllPayloadTypes(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "paths.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	// A GroupText packet relayed over two hops — no advert, so the old scan
	// discarded it entirely.
	raw := "15" + "82" + "AAAAAA" + "BBBBBB" + "C6DEAD" + "0011223344556677"
	pkt, err := meshcore.DecodeHex(raw)
	if err != nil || pkt == nil || pkt.Advert != nil || len(pkt.Path) != 2 {
		t.Fatalf("fixture is not a 2-hop non-advert packet: %v", err)
	}

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		if err := st.Record(store.Observation{
			Packet: pkt, RawHex: raw, ObserverID: "obs-a",
			ReceivedAt: now.Add(-time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	rep, err := DetectInjection(st, nil, now.Add(-6*time.Hour).Format(time.RFC3339Nano), 0)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if rep.PacketsScanned != 5 {
		t.Errorf("PacketsScanned = %d, want 5", rep.PacketsScanned)
	}
	if rep.PathsScanned != 5 {
		t.Errorf("PathsScanned = %d, want 5 — non-advert packets must contribute path evidence",
			rep.PathsScanned)
	}
	if rep.AdvertsScanned != 0 {
		t.Errorf("AdvertsScanned = %d, want 0 (nothing here is an advert)", rep.AdvertsScanned)
	}
	// Both hops are unknown to this store, so they must count as unresolved rather
	// than silently joining into a fabricated adjacency.
	if rep.UnresolvedHops != 10 {
		t.Errorf("UnresolvedHops = %d, want 10", rep.UnresolvedHops)
	}
}
