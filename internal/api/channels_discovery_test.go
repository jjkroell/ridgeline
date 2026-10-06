package api

import (
	"encoding/hex"
	"testing"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

func TestChannelCandidateEndpoint(t *testing.T) {
	st, base, cleanup := newAuthEnv(t)
	defer cleanup()

	owner := newClient(t, base)
	owner.do("POST", "/api/auth/register",
		map[string]string{"email": "owner@example.com", "password": "hunter2hunter2"}, false)
	member := newClient(t, base)
	member.do("POST", "/api/auth/register",
		map[string]string{"email": "m@example.com", "password": "hunter2hunter2", "displayName": "M"}, false)
	anon := newClient(t, base)

	// Submission is open to anonymous callers (browser adds feed the pool), but
	// still validates and is rate-limited.
	if resp, _ := anon.do("POST", "/api/channels/candidates", map[string]string{"name": "weather"}, true); resp.StatusCode != 200 {
		t.Errorf("anonymous candidate submission: got %d, want 200", resp.StatusCode)
	}

	// Junk names are rejected.
	for _, bad := range []string{"", "#", "has space", "two#tags"} {
		if resp, _ := member.do("POST", "/api/channels/candidates", map[string]string{"name": bad}, true); resp.StatusCode != 400 {
			t.Errorf("name %q: got %d, want 400", bad, resp.StatusCode)
		}
	}

	// A good name is normalized (leading '#' stripped) and accepted.
	resp, body := member.do("POST", "/api/channels/candidates", map[string]string{"name": "#BC-Fire"}, true)
	if resp.StatusCode != 200 || body["name"] != "BC-Fire" {
		t.Fatalf("submit: %d %v", resp.StatusCode, body)
	}

	// It shows as pending, not yet confirmed (no traffic proved it).
	_, disc := member.do("GET", "/api/channels/discovered", nil, false)
	if disc["pending"].(float64) != 2 || disc["confirmed"].(float64) != 0 {
		t.Fatalf("after submits want pending=2 confirmed=0, got %v", disc)
	}
	if chs, _ := disc["channels"].([]any); len(chs) != 0 {
		t.Errorf("a pending candidate must not appear as a confirmed channel: %v", chs)
	}

	// Simulate the sweep confirming it (store-level discovery is tested separately).
	key := meshcore.DeriveHashtagKey("BC-Fire")
	if err := st.ConfirmChannel("BC-Fire", hex.EncodeToString(key)); err != nil {
		t.Fatal(err)
	}
	_, disc = member.do("GET", "/api/channels/discovered", nil, false)
	if disc["confirmed"].(float64) != 1 {
		t.Fatalf("after confirm want confirmed=1, got %v", disc)
	}
	chs, _ := disc["channels"].([]any)
	if len(chs) != 1 {
		t.Fatalf("confirmed channel not listed: %v", disc["channels"])
	}
	if first, _ := chs[0].(map[string]any); first["name"] != "BC-Fire" {
		t.Errorf("listed channel = %v, want name BC-Fire", chs[0])
	}
}
