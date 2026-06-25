package analytics

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

func TestConfidentMode(t *testing.T) {
	cases := []struct {
		name   string
		votes  map[int]int
		want   int
		wantOK bool
	}{
		{"clear majority", map[int]int{1: 5, 3: 1}, 1, true},
		{"single corrupt outlier loses", map[int]int{3: 9, 1: 1}, 3, true},
		{"lone vote not trusted", map[int]int{2: 1}, 0, false},
		{"two-way tie has no majority", map[int]int{1: 3, 3: 3}, 0, false},
		{"empty", map[int]int{}, 0, false},
		{"two equal but one dominates total", map[int]int{1: 3, 2: 2}, 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := confidentMode(c.votes)
			if got != c.want || ok != c.wantOK {
				t.Fatalf("confidentMode(%v) = (%d,%v), want (%d,%v)", c.votes, got, ok, c.want, c.wantOK)
			}
		})
	}
}

// TestConsensusHashSizes records several genuine adverts plus one with a
// corrupted path-length byte (reporting a different size) for the same node, and
// verifies the vote recovers the true size and ignores the outlier.
func TestConsensusHashSizes(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "hs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	adv, err := meshcore.DecodeHex(advertFixture)
	if err != nil || adv.Advert == nil {
		t.Fatalf("decode advert: %v", err)
	}
	pubkey := adv.Advert.PublicKey
	if adv.PathHashSize != 1 {
		t.Fatalf("fixture PathHashSize = %d, want 1", adv.PathHashSize)
	}

	// Same packet with the path-length byte flipped 0x00 -> 0x80 (bits7:6 = 2 =>
	// hash size 3, hop count still 0): a corrupt advert for the SAME node that
	// votes the wrong size, while the pubkey and payload are unchanged.
	corruptHex := "1180" + advertFixture[4:]
	corrupt, err := meshcore.DecodeHex(corruptHex)
	if err != nil || corrupt.Advert == nil {
		t.Fatalf("decode corrupt advert: %v", err)
	}
	if corrupt.PathHashSize != 3 || corrupt.Advert.PublicKey != pubkey {
		t.Fatalf("corrupt PathHashSize=%d pubkey=%q, want 3 / %q", corrupt.PathHashSize, corrupt.Advert.PublicKey, pubkey)
	}

	now := time.Now().UTC()
	rec := func(p *meshcore.Packet, hex, obs string, ago time.Duration) {
		if err := st.Record(store.Observation{
			Packet: p, RawHex: hex, ObserverID: obs, Region: "YVR",
			ReceivedAt: now.Add(-ago),
		}); err != nil {
			t.Fatal(err)
		}
	}
	rec(adv, advertFixture, "obs-A", 30*time.Minute)
	rec(adv, advertFixture, "obs-B", 20*time.Minute)
	rec(adv, advertFixture, "obs-C", 10*time.Minute)
	rec(corrupt, corruptHex, "obs-D", 5*time.Minute) // the corrupt outlier

	cutoff := now.Add(-time.Hour).Format(time.RFC3339Nano)
	consensus, err := ConsensusHashSizes(st, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if got := consensus[pubkey]; got != 1 {
		t.Fatalf("consensus hash size = %d, want 1 (corrupt 3 outvoted)", got)
	}
}

// TestConsensusHashSizesPerTransmission proves the vote counts one vote per
// transmission, not per received copy: a single corrupt transmission heard by
// many observers (a reflood burst) must NOT outweigh fewer genuine
// transmissions. Counting raw copies here would be 12 corrupt vs 2 genuine and
// pick the wrong size.
func TestConsensusHashSizesPerTransmission(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "hsx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	adv, _ := meshcore.DecodeHex(advertFixture)
	pubkey := adv.Advert.PublicKey
	corruptHex := "1180" + advertFixture[4:]
	corrupt, _ := meshcore.DecodeHex(corruptHex)

	now := time.Now().UTC()
	rec := func(p *meshcore.Packet, hex, obs string, ago time.Duration) {
		if err := st.Record(store.Observation{
			Packet: p, RawHex: hex, ObserverID: obs, Region: "YVR",
			ReceivedAt: now.Add(-ago),
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Two genuine transmissions, days apart (one observer each) → 2 votes for 1.
	rec(adv, advertFixture, "obs-A", 5*24*time.Hour)
	rec(adv, advertFixture, "obs-B", 2*24*time.Hour)
	// One corrupt transmission heard by 12 observers within seconds (a reflood
	// burst) → a single vote for 3, not twelve.
	for i := 0; i < 12; i++ {
		rec(corrupt, corruptHex, "obsC"+string(rune('a'+i)), time.Hour+time.Duration(i)*time.Second)
	}

	cutoff := now.Add(-7 * 24 * time.Hour).Format(time.RFC3339Nano)
	consensus, err := ConsensusHashSizes(st, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if got := consensus[pubkey]; got != 1 {
		t.Fatalf("consensus hash size = %d, want 1 (one corrupt burst must not outvote two genuine transmissions)", got)
	}
}
