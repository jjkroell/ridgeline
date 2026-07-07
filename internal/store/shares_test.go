package store

import "testing"

func TestLocationShares(t *testing.T) {
	st := testStore(t)
	owner, _ := st.CreateUser("owner@example.com", "h", "Owner")
	alice, _ := st.CreateUser("alice@example.com", "h", "Alice")
	bob, _ := st.CreateUser("bob@example.com", "h", "")

	// No shares initially.
	if has, _ := st.HasLocationShare(locNode, alice.ID); has {
		t.Fatal("alice should not have a share yet")
	}

	// Grant to alice + bob.
	if err := st.ShareLocation(locNode, owner.ID, alice.ID); err != nil {
		t.Fatalf("share alice: %v", err)
	}
	if err := st.ShareLocation(locNode, owner.ID, bob.ID); err != nil {
		t.Fatalf("share bob: %v", err)
	}

	if has, _ := st.HasLocationShare(locNode, alice.ID); !has {
		t.Error("alice should have a share")
	}
	if has, _ := st.HasLocationShare(locNode, bob.ID); !has {
		t.Error("bob should have a share")
	}

	// List shows both, with display name falling back to email.
	shares, err := st.ListLocationShares(locNode)
	if err != nil || len(shares) != 2 {
		t.Fatalf("expected 2 shares, got %d err=%v", len(shares), err)
	}
	byID := map[int64]LocationShare{}
	for _, sh := range shares {
		byID[sh.GranteeUserID] = sh
	}
	if byID[alice.ID].DisplayName != "Alice" {
		t.Errorf("alice display name = %q", byID[alice.ID].DisplayName)
	}
	if byID[bob.ID].DisplayName != "bob@example.com" {
		t.Errorf("bob (no display name) should fall back to email, got %q", byID[bob.ID].DisplayName)
	}

	// Re-granting is idempotent (no duplicate row).
	if err := st.ShareLocation(locNode, owner.ID, alice.ID); err != nil {
		t.Fatalf("re-share: %v", err)
	}
	shares, _ = st.ListLocationShares(locNode)
	if len(shares) != 2 {
		t.Fatalf("re-grant should not duplicate, got %d", len(shares))
	}

	// Revoke alice.
	removed, err := st.UnshareLocation(locNode, alice.ID)
	if err != nil || !removed {
		t.Fatalf("unshare alice: removed=%v err=%v", removed, err)
	}
	if has, _ := st.HasLocationShare(locNode, alice.ID); has {
		t.Error("alice share should be gone")
	}

	// Drop all (ownership release path).
	if err := st.DeleteLocationShares(locNode); err != nil {
		t.Fatalf("delete all: %v", err)
	}
	if shares, _ := st.ListLocationShares(locNode); len(shares) != 0 {
		t.Errorf("expected no shares after delete-all, got %d", len(shares))
	}
}
