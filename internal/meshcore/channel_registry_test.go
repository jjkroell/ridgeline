package meshcore

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// buildGroupTextPayload constructs a valid group-text payload for a channel key:
// channel_hash(1) | MAC(2) | AES-128-ECB(timestamp|flags|"sender: msg"). It is
// the inverse of decryptGroupText, so a round-trip proves the derivation and the
// verifier agree.
func buildGroupTextPayload(key []byte, sender, msg string) []byte {
	body := make([]byte, 5)
	binary.LittleEndian.PutUint32(body[0:4], 1700000000)
	body[4] = 0
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
	out := []byte{channelHashByte(key), sum[0], sum[1]}
	return append(out, ct...)
}

func TestDeriveHashtagKeyMatchesFirmware(t *testing.T) {
	// SHA256("#"+name)[:16]. Pinned to a known vector so a refactor can't drift.
	got := hex.EncodeToString(DeriveHashtagKey("weather"))
	want := sha256.Sum256([]byte("#weather"))
	if got != hex.EncodeToString(want[:16]) {
		t.Fatalf("DeriveHashtagKey(weather) = %s, want %s", got, hex.EncodeToString(want[:16]))
	}
	// Case-sensitive, verbatim: different case is a different channel.
	if hex.EncodeToString(DeriveHashtagKey("Weather")) == got {
		t.Error("derivation must be case-sensitive")
	}
}

func TestVerifyGroupTextIsAnOracle(t *testing.T) {
	key := DeriveHashtagKey("weather")
	payload := buildGroupTextPayload(key, "alice", "hello mesh")

	ts, sender, msg, ok := VerifyGroupText(payload, key)
	if !ok {
		t.Fatal("correct key failed to verify its own payload")
	}
	if sender != "alice" || msg != "hello mesh" || ts != 1700000000 {
		t.Errorf("decrypt = ts:%d %q/%q, want 1700000000 alice/hello mesh", ts, sender, msg)
	}

	// A wrong key must NOT verify — this is the whole safety of the sweep.
	if _, _, _, ok := VerifyGroupText(payload, DeriveHashtagKey("notit")); ok {
		t.Error("a wrong key verified a payload — the MAC oracle is broken")
	}
	// A key whose hash byte happens to match but whose MAC won't: cheap-reject path.
	bad := make([]byte, 16)
	copy(bad, key)
	bad[15] ^= 0xFF
	if _, _, _, ok := VerifyGroupText(payload, bad); ok {
		t.Error("a near-miss key verified")
	}
}

func TestRegisterChannelMakesDecoderDecrypt(t *testing.T) {
	name := "regtest-" + t.Name()
	key := DeriveHashtagKey(name)
	payload := buildGroupTextPayload(key, "bob", "registered!")

	// Before registration the decoder sees only the hash + MAC.
	gt := decodeGroupText(payload)
	if gt == nil || gt.Decrypted {
		t.Fatalf("payload decrypted before its channel was registered: %+v", gt)
	}

	RegisterChannel(name, key)
	gt = decodeGroupText(payload)
	if gt == nil || !gt.Decrypted || gt.Channel != name || gt.Message != "registered!" {
		t.Fatalf("decoder did not decrypt after RegisterChannel: %+v", gt)
	}

	// Idempotent by name.
	before := len(RegisteredChannelNames())
	RegisterChannel(name, key)
	if after := len(RegisteredChannelNames()); after != before {
		t.Errorf("RegisterChannel not idempotent: %d -> %d", before, after)
	}
}
