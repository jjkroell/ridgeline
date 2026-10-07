package store

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

// groupTextRaw builds a full flood GroupText packet (hex) for a channel key:
// header(0x15) | pathlen(0x00) | [channel_hash(1) | MAC(2) | AES-ECB(ts|flags|"s: m")].
func groupTextRaw(t *testing.T, key []byte, sender, msg string) string {
	t.Helper()
	body := make([]byte, 5)
	binary.LittleEndian.PutUint32(body[0:4], uint32(time.Now().Unix()))
	body = append(body, []byte(sender+": "+msg)...)
	for len(body)%aes.BlockSize != 0 {
		body = append(body, 0)
	}
	block, _ := aes.NewCipher(key)
	ct := make([]byte, len(body))
	for i := 0; i < len(body); i += aes.BlockSize {
		block.Encrypt(ct[i:i+aes.BlockSize], body[i:i+aes.BlockSize])
	}
	secret := make([]byte, 32)
	copy(secret, key)
	m := hmac.New(sha256.New, secret)
	m.Write(ct)
	sum := m.Sum(nil)
	h := sha256.Sum256(key)
	payload := append([]byte{h[0], sum[0], sum[1]}, ct...)
	pkt := append([]byte{0x15, 0x00}, payload...)
	return hex.EncodeToString(pkt)
}

func recordRaw(t *testing.T, st *Store, raw string, at time.Time) {
	t.Helper()
	pkt, err := meshcore.DecodeHex(raw)
	if err != nil || pkt == nil {
		t.Fatalf("decode synthetic packet: %v", err)
	}
	if err := st.Record(Observation{Packet: pkt, RawHex: raw, ReceivedAt: at}); err != nil {
		t.Fatalf("record: %v", err)
	}
}

// TestDiscoverChannelsConfirmsAndHarvests is the end-to-end discovery test:
// a user-submitted candidate gets confirmed from real traffic, and a #tag
// mentioned in a decryptable Public message is harvested and confirmed in the
// same pass — all proven by decryption, never guessed.
func TestDiscoverChannelsConfirmsAndHarvests(t *testing.T) {
	st := testStore(t)
	now := time.Now().UTC()
	pubKey, _ := hex.DecodeString("8b3387e9c5cdea6ac9e5edbaa115cd72")

	// Traffic on a secret channel the operator has NOT named yet.
	secretKey := meshcore.DeriveHashtagKey("secret")
	recordRaw(t, st, groupTextRaw(t, secretKey, "alice", "anyone on secret?"), now)
	recordRaw(t, st, groupTextRaw(t, secretKey, "bob", "yep"), now)

	// Traffic on a channel nobody submitted, but referenced in a Public message.
	weatherKey := meshcore.DeriveHashtagKey("weather")
	recordRaw(t, st, groupTextRaw(t, weatherKey, "carol", "storm coming"), now)
	recordRaw(t, st, groupTextRaw(t, pubKey, "dave", "chatter is over on #weather btw"), now)

	// Noise: a channel with no candidate and no mention must stay unknown.
	ghostKey := meshcore.DeriveHashtagKey("ghost-xyzzy")
	recordRaw(t, st, groupTextRaw(t, ghostKey, "eve", "invisible"), now)

	// A user submits "#secret" from their browser.
	uid := int64(7)
	if n, err := st.AddChannelCandidate("#secret", ChannelSourceUser, &uid); err != nil || n != "secret" {
		t.Fatalf("AddChannelCandidate = %q, %v", n, err)
	}

	confirmed, err := st.DiscoverChannels(now.Add(-time.Hour).Format(time.RFC3339Nano), 0)
	if err != nil {
		t.Fatalf("DiscoverChannels: %v", err)
	}

	got := map[string]bool{}
	for _, c := range confirmed {
		got[c] = true
	}
	if !got["secret"] {
		t.Error("user-submitted #secret was not confirmed from its traffic")
	}
	if !got["weather"] {
		t.Error("#weather referenced in a Public message was not harvested and confirmed")
	}
	if got["ghost-xyzzy"] {
		t.Error("a channel with no candidate and no mention was confirmed — impossible without the key")
	}

	// Confirmed channels persist with their key and register with the decoder.
	chans, _ := st.ConfirmedChannels()
	names := map[string]string{}
	for _, c := range chans {
		names[c.Name] = c.KeyHex
	}
	if names["secret"] != hex.EncodeToString(secretKey) {
		t.Errorf("stored key for secret = %q, want %s", names["secret"], hex.EncodeToString(secretKey))
	}
	reg := map[string]bool{}
	for _, n := range meshcore.RegisteredChannelNames() {
		reg[n] = true
	}
	if !reg["secret"] || !reg["weather"] {
		t.Error("confirmed channels were not registered with the decoder")
	}

	// The in-memory list the endpoint serves matches the database, with no
	// reload, and pending excludes what just confirmed.
	cached, pending, nConf := st.CachedConfirmedChannels()
	if nConf != len(chans) || len(cached) != len(chans) {
		t.Errorf("cached %d confirmed, database has %d", nConf, len(chans))
	}
	if dbPending, _, _ := st.CountChannelCandidates(); pending != dbPending {
		t.Errorf("cached pending = %d, database has %d", pending, dbPending)
	}

	// A fresh process seeds the same list from the database at startup.
	st.confirmedChans = nil
	if _, err := st.LoadConfirmedChannels(); err != nil {
		t.Fatalf("LoadConfirmedChannels: %v", err)
	}
	if reloaded, _, _ := st.CachedConfirmedChannels(); len(reloaded) != len(chans) {
		t.Errorf("startup load cached %d, want %d", len(reloaded), len(chans))
	}

	// Idempotent: a second pass confirms nothing new.
	again, err := st.DiscoverChannels(now.Add(-time.Hour).Format(time.RFC3339Nano), 0)
	if err != nil {
		t.Fatalf("second DiscoverChannels: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("second pass re-confirmed %v, want none", again)
	}
}

func TestNormalizeChannelNameRejectsJunk(t *testing.T) {
	ok := map[string]string{"#weather": "weather", "  #BC-Fire ": "BC-Fire", "general": "general"}
	for in, want := range ok {
		if _, err := testStore(t).AddChannelCandidate(in, ChannelSourceUser, nil); err != nil {
			t.Fatalf("add %q: %v", in, err)
		}
		if got := normalizeChannelName(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "#", "   ", "#has space", "two#tags", string(make([]byte, 40))} {
		if got := normalizeChannelName(bad); got != "" {
			t.Errorf("normalize(%q) = %q, want rejected", bad, got)
		}
	}
}
