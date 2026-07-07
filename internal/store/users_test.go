package store

import (
	"testing"
	"time"
)

func TestCreateUserBootstrapsFirstAdmin(t *testing.T) {
	st := testStore(t)

	u1, err := st.CreateUser("Owner@Example.com", "hash1", "Owner")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if u1.Email != "owner@example.com" {
		t.Errorf("email should be lowercased, got %q", u1.Email)
	}
	if !u1.IsAdmin || !u1.CanClaim || !u1.IsOwner {
		t.Error("first user should be bootstrapped as admin + can_claim + owner")
	}

	u2, err := st.CreateUser("second@example.com", "hash2", "")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if u2.IsAdmin || u2.CanClaim || u2.IsOwner {
		t.Error("subsequent users must not be admin/can_claim/owner by default")
	}
}

func TestBlockUserVoidsSessions(t *testing.T) {
	st := testStore(t)
	st.CreateUser("owner@example.com", "h", "") // first = owner
	u, _ := st.CreateUser("blocked@example.com", "h", "")
	st.CreateSession("hash-b", u.ID, "csrf", time.Hour)

	// Block + invalidate: the session must stop resolving.
	if err := st.SetUserBlocked(u.ID, true); err != nil {
		t.Fatalf("block: %v", err)
	}
	st.DeleteUserSessions(u.ID)
	if _, _, ok, _ := st.SessionUser("hash-b"); ok {
		t.Error("blocked user's session should not resolve")
	}
	// Even a fresh session must not resolve while blocked.
	st.CreateSession("hash-b2", u.ID, "csrf", time.Hour)
	if _, _, ok, _ := st.SessionUser("hash-b2"); ok {
		t.Error("a blocked account must not resolve any session")
	}
	// Unblock restores access.
	st.SetUserBlocked(u.ID, false)
	st.CreateSession("hash-b3", u.ID, "csrf", time.Hour)
	if _, _, ok, _ := st.SessionUser("hash-b3"); !ok {
		t.Error("unblocked account should resolve again")
	}
}

func TestDeleteUser(t *testing.T) {
	st := testStore(t)
	st.CreateUser("owner@example.com", "h", "")
	u, _ := st.CreateUser("gone@example.com", "h", "")
	st.CreateSession("hash-g", u.ID, "csrf", time.Hour)

	if err := st.DeleteUser(u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok, _ := st.GetUserByID(u.ID); ok {
		t.Error("deleted user should not be found")
	}
	if _, _, ok, _ := st.SessionUser("hash-g"); ok {
		t.Error("deleted user's session should be gone")
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	st := testStore(t)
	if _, err := st.CreateUser("dup@example.com", "h", ""); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := st.CreateUser("DUP@example.com", "h2", ""); err != ErrEmailTaken {
		t.Errorf("expected ErrEmailTaken for case-insensitive dup, got %v", err)
	}
}

func TestGetUserByEmailAndID(t *testing.T) {
	st := testStore(t)
	created, _ := st.CreateUser("a@example.com", "h", "A")

	got, ok, err := st.GetUserByEmail("A@EXAMPLE.COM")
	if err != nil || !ok {
		t.Fatalf("by email: ok=%v err=%v", ok, err)
	}
	if got.ID != created.ID || got.PasswordHash != "h" {
		t.Error("by-email lookup returned wrong row")
	}
	if _, ok, _ := st.GetUserByEmail("nobody@example.com"); ok {
		t.Error("missing email should return ok=false")
	}
	if _, ok, _ := st.GetUserByID(created.ID); !ok {
		t.Error("by-id lookup failed")
	}
}

func TestSetUserFlags(t *testing.T) {
	st := testStore(t)
	st.CreateUser("owner@example.com", "h", "") // first = admin
	u, _ := st.CreateUser("member@example.com", "h", "")
	if err := st.SetUserFlags(u.ID, false, true); err != nil {
		t.Fatalf("set flags: %v", err)
	}
	got, _, _ := st.GetUserByID(u.ID)
	if got.IsAdmin || !got.CanClaim {
		t.Errorf("flags not applied: admin=%v canClaim=%v", got.IsAdmin, got.CanClaim)
	}
}

func TestSessionLifecycle(t *testing.T) {
	st := testStore(t)
	u, _ := st.CreateUser("s@example.com", "h", "")

	if err := st.CreateSession("hash-abc", u.ID, "csrf-1", time.Hour); err != nil {
		t.Fatalf("create session: %v", err)
	}
	sess, gotUser, ok, err := st.SessionUser("hash-abc")
	if err != nil || !ok {
		t.Fatalf("resolve session: ok=%v err=%v", ok, err)
	}
	if gotUser.ID != u.ID || sess.CSRF != "csrf-1" {
		t.Error("session resolved to wrong user/csrf")
	}

	if err := st.DeleteSession("hash-abc"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, _, ok, _ := st.SessionUser("hash-abc"); ok {
		t.Error("deleted session should not resolve")
	}
}

func TestSessionExpiryAndPrune(t *testing.T) {
	st := testStore(t)
	u, _ := st.CreateUser("e@example.com", "h", "")

	// Already-expired session: resolves as not-ok and is dropped on use.
	if err := st.CreateSession("hash-exp", u.ID, "csrf", -time.Minute); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, _, ok, _ := st.SessionUser("hash-exp"); ok {
		t.Error("expired session must not resolve")
	}

	// Prune removes expired rows below the cutoff.
	st.CreateSession("hash-old", u.ID, "csrf", -time.Hour)
	st.CreateSession("hash-live", u.ID, "csrf", time.Hour)
	n, err := st.PruneSessions(time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n < 1 {
		t.Errorf("expected to prune at least the expired row, pruned %d", n)
	}
	if _, _, ok, _ := st.SessionUser("hash-live"); !ok {
		t.Error("live session should survive prune")
	}
}
