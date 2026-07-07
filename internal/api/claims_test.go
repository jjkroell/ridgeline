package api

import (
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// A real captured advert; Record it so the claimed node exists in the store.
const claimAdvertHex = "10B76000008A654F144C4A43D07F024E8E0A59120F9A3C2E825453F88F861C48C2F2BA245CD672C56C5BE3F52CB4470337904147543724983C2978DCCE234CE41898674714704C6A642E674B47350417E441B17CF5CC4BF444792266EDC3F4FE7970F4DF977CA9344CE16A45FEA7FCD25A85E9653FD2F1DB63666B0A792290A37F398341E70B61099279FFED021081B5F8F09F8D924368657272792048696C6C20F09F8D92"

func TestClaimFlow(t *testing.T) {
	st, base, cleanup := newAuthEnv(t)
	defer cleanup()

	// Seed the node by recording a real advert.
	pkt, err := meshcore.DecodeHex(claimAdvertHex)
	if err != nil || pkt.Advert == nil {
		t.Fatalf("decode advert: %v", err)
	}
	node := pkt.Advert.PublicKey
	if err := st.Record(store.Observation{Packet: pkt, RawHex: claimAdvertHex, ReceivedAt: time.Now()}); err != nil {
		t.Fatalf("record node: %v", err)
	}

	owner := newClient(t, base)
	owner.do("POST", "/api/auth/register",
		map[string]string{"email": "owner@example.com", "password": "hunter2hunter2"}, false)

	member := newClient(t, base)
	_, mb := member.do("POST", "/api/auth/register",
		map[string]string{"email": "member@example.com", "password": "hunter2hunter2", "displayName": "Member"}, false)
	memberID := int64(member.user(mb)["id"].(float64))

	// Not-yet-approved member cannot claim.
	if resp, _ := member.do("POST", "/api/claims", map[string]string{"pubkey": node}, true); resp.StatusCode != 403 {
		t.Errorf("unapproved claim should be 403, got %d", resp.StatusCode)
	}

	// Owner grants can_claim; member refreshes.
	owner.do("POST", "/api/admin/users/flags",
		map[string]any{"id": memberID, "isAdmin": false, "canClaim": true}, true)
	member.do("GET", "/api/auth/me", nil, false)

	// Invalid + unknown pubkeys.
	if resp, _ := member.do("POST", "/api/claims", map[string]string{"pubkey": "xyz"}, true); resp.StatusCode != 400 {
		t.Errorf("bad pubkey should be 400, got %d", resp.StatusCode)
	}
	unknown := "00000000000000000000000000000000000000000000000000000000000000AA"
	if resp, _ := member.do("POST", "/api/claims", map[string]string{"pubkey": unknown}, true); resp.StatusCode != 404 {
		t.Errorf("unknown node should be 404, got %d", resp.StatusCode)
	}

	// Approved member opens a claim → gets a code.
	resp, cb := member.do("POST", "/api/claims", map[string]string{"pubkey": node}, true)
	if resp.StatusCode != 200 {
		t.Fatalf("claim create: %d body %v", resp.StatusCode, cb)
	}
	code, _ := cb["code"].(string)
	if code == "" || cb["status"] != "pending" {
		t.Fatalf("expected a pending claim with a code, got %v", cb)
	}

	// Status endpoint shows the caller's pending claim, no owner yet.
	_, cs := member.do("GET", "/api/nodes/"+node+"/claim", nil, false)
	if cs["owner"] != nil || cs["ownedByMe"] != false {
		t.Errorf("node should be unowned pre-verification, got %v", cs)
	}
	if mine, _ := cs["mine"].(map[string]any); mine == nil || mine["status"] != "pending" {
		t.Errorf("expected caller's pending claim in status, got %v", cs["mine"])
	}

	// Simulate the ingest verifier seeing a signature-valid advert whose name
	// carries the code (the ingest hook calls exactly this after the sig check).
	if v, err := st.VerifyPendingClaims(node, "MyRepeater "+code); err != nil || len(v) != 1 {
		t.Fatalf("verify claim: n=%d err=%v", len(v), err)
	}

	// Now the member owns it; a public status call shows the owner's name.
	anon := newClient(t, base)
	_, ps := anon.do("GET", "/api/nodes/"+node+"/claim", nil, false)
	owner2, _ := ps["owner"].(map[string]any)
	if owner2 == nil || owner2["displayName"] != "Member" {
		t.Errorf("public status should show owner display name, got %v", ps["owner"])
	}
	_, ms := member.do("GET", "/api/nodes/"+node+"/claim", nil, false)
	if ms["ownedByMe"] != true {
		t.Error("member should see ownedByMe=true")
	}

	// Someone else can't claim an owned node.
	if resp, _ := owner.do("POST", "/api/claims", map[string]string{"pubkey": node}, true); resp.StatusCode != 409 {
		t.Errorf("claiming an owned node should be 409, got %d", resp.StatusCode)
	}

	// Owner list works, and release frees the node.
	if resp, _ := member.do("GET", "/api/claims/mine", nil, false); resp.StatusCode != 200 {
		t.Errorf("claims/mine should be 200, got %d", resp.StatusCode)
	}
	if resp, _ := member.do("DELETE", "/api/claims/"+node, nil, true); resp.StatusCode != 200 {
		t.Errorf("release should be 200, got %d", resp.StatusCode)
	}
	_, after := anon.do("GET", "/api/nodes/"+node+"/claim", nil, false)
	if after["owner"] != nil {
		t.Error("node should be unowned after release")
	}
}
