package api

import (
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// TestAdminClaimGrantsOwnershipWithoutProof covers the admin shortcut to
// ownership. The two normal routes both prove possession of the node (rename it,
// or sign with its private key); this one proves nothing, so the gate and the
// audit trail are the whole safety story and both are asserted here.
func TestAdminClaimGrantsOwnershipWithoutProof(t *testing.T) {
	st, base, cleanup := newAuthEnv(t)
	defer cleanup()

	pkt, err := meshcore.DecodeHex(claimAdvertHex)
	if err != nil || pkt.Advert == nil {
		t.Fatalf("decode advert: %v", err)
	}
	node := pkt.Advert.PublicKey
	if err := st.Record(store.Observation{Packet: pkt, RawHex: claimAdvertHex, ReceivedAt: time.Now()}); err != nil {
		t.Fatalf("record node: %v", err)
	}

	// First account registered is the admin/owner of the deployment.
	admin := newClient(t, base)
	admin.do("POST", "/api/auth/register",
		map[string]string{"email": "admin@example.com", "password": "hunter2hunter2"}, false)

	member := newClient(t, base)
	member.do("POST", "/api/auth/register",
		map[string]string{"email": "member@example.com", "password": "hunter2hunter2", "displayName": "Member"}, false)

	anon := newClient(t, base)

	// --- the gate ---
	if resp, _ := member.do("POST", "/api/admin/claims", map[string]string{"pubkey": node}, true); resp.StatusCode != 403 {
		t.Errorf("a non-admin member must not reach the admin claim: got %d, want 403", resp.StatusCode)
	}
	if resp, _ := anon.do("POST", "/api/admin/claims", map[string]string{"pubkey": node}, true); resp.StatusCode == 200 {
		t.Error("an anonymous caller reached the admin claim")
	}
	// The gate must not have granted anything as a side effect.
	if _, ok, _ := st.NodeOwner(node); ok {
		t.Fatal("node gained an owner from a refused request")
	}

	// --- validation, same contract as the normal claim ---
	if resp, _ := admin.do("POST", "/api/admin/claims", map[string]string{"pubkey": "xyz"}, true); resp.StatusCode != 400 {
		t.Errorf("bad pubkey: got %d, want 400", resp.StatusCode)
	}
	unknown := "00000000000000000000000000000000000000000000000000000000000000AA"
	if resp, _ := admin.do("POST", "/api/admin/claims", map[string]string{"pubkey": unknown}, true); resp.StatusCode != 404 {
		t.Errorf("unknown node: got %d, want 404", resp.StatusCode)
	}

	// --- the grant: verified immediately, with no code to redeem ---
	resp, body := admin.do("POST", "/api/admin/claims", map[string]string{"pubkey": node}, true)
	if resp.StatusCode != 200 {
		t.Fatalf("admin claim: %d body %v", resp.StatusCode, body)
	}
	if body["status"] != "verified" {
		t.Errorf("claim status = %v, want verified — the point is to skip the pending step", body["status"])
	}
	if code, _ := body["code"].(string); code != "" {
		t.Errorf("a granted claim must carry no verification code, got %q", code)
	}
	if st.HasPendingClaim(node) {
		t.Error("node still reads as having a pending claim")
	}
	owner, ok, err := st.NodeOwner(node)
	if err != nil || !ok {
		t.Fatalf("NodeOwner after grant: ok=%v err=%v", ok, err)
	}

	// --- idempotent: re-granting is not an error and does not duplicate ---
	if resp, _ := admin.do("POST", "/api/admin/claims", map[string]string{"pubkey": node}, true); resp.StatusCode != 200 {
		t.Errorf("re-granting own claim: got %d, want 200", resp.StatusCode)
	}
	if again, _, _ := st.NodeOwner(node); again.UserID != owner.UserID {
		t.Error("owner changed on a repeat grant")
	}

	// --- it is audit-logged: ownership granted rather than earned must be
	//     distinguishable after the fact ---
	entries, err := st.ListAudit(node)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Action == "admin_claim" {
			found = true
			if e.ActorEmail != "admin@example.com" {
				t.Errorf("audit actor = %q, want the admin", e.ActorEmail)
			}
		}
	}
	if !found {
		t.Errorf("no admin_claim audit entry for the granted node, got %+v", entries)
	}
}

// TestAdminClaimWillNotTakeANodeFromAnotherUser pins the deliberate limit: an
// admin fills an ownership vacuum, it does not seize. Taking a node off someone
// is a louder action with its own tooling, and silently overriding here would
// make the shortcut unsafe to reach for.
func TestAdminClaimWillNotTakeANodeFromAnotherUser(t *testing.T) {
	st, base, cleanup := newAuthEnv(t)
	defer cleanup()

	pkt, _ := meshcore.DecodeHex(claimAdvertHex)
	node := pkt.Advert.PublicKey
	if err := st.Record(store.Observation{Packet: pkt, RawHex: claimAdvertHex, ReceivedAt: time.Now()}); err != nil {
		t.Fatalf("record node: %v", err)
	}

	admin := newClient(t, base)
	admin.do("POST", "/api/auth/register",
		map[string]string{"email": "admin@example.com", "password": "hunter2hunter2"}, false)

	// A member legitimately owns the node already.
	u, err := st.CreateUser("holder@example.com", "h", "Holder")
	if err != nil {
		t.Fatalf("create holder: %v", err)
	}
	if _, err := st.CreateVerifiedClaim(node, u.ID); err != nil {
		t.Fatalf("seed ownership: %v", err)
	}

	resp, _ := admin.do("POST", "/api/admin/claims", map[string]string{"pubkey": node}, true)
	if resp.StatusCode != 409 {
		t.Errorf("claiming an owned node: got %d, want 409", resp.StatusCode)
	}
	if owner, ok, _ := st.NodeOwner(node); !ok || owner.UserID != u.ID {
		t.Error("the original owner must be untouched by a refused admin claim")
	}
}
