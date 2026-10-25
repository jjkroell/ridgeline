package store

import "testing"

// A reset or verification token mailed before an email change must not work
// after it: completing a reset would mark the new, unconfirmed address verified.
func TestEmailChangeVoidsOutstandingTokens(t *testing.T) {
	st := testStore(t)
	u, err := st.CreateUser("old@example.com", "x", "")
	if err != nil {
		t.Fatal(err)
	}
	reset, err := st.CreatePasswordReset(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateEmailVerification(u.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateEmail(u.ID, "new@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := st.ConsumePasswordReset(reset); ok {
		t.Fatal("reset token from before the email change still works")
	}
	var n int
	st.db.QueryRow(`SELECT COUNT(*) FROM email_verifications WHERE user_id = ?`, u.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("want old verification tokens gone, %d remain", n)
	}
}
