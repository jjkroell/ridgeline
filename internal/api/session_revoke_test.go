package api

import "testing"

// Changing the password or email signs the account out of every other
// session, while the session that made the change stays signed in.
func TestAccountChangesRevokeOtherSessions(t *testing.T) {
	_, base, cleanup := newAuthEnv(t)
	defer cleanup()
	creds := map[string]string{"email": "me@example.com", "password": "hunter2hunter2"}
	a := newClient(t, base)
	a.do("POST", "/api/auth/register", creds, false)

	signedIn := func(c *authTestClient) bool {
		_, body := c.do("GET", "/api/auth/me", nil, false)
		return body["user"] != nil
	}
	for _, step := range []struct {
		path string
		body map[string]string
	}{
		{"/api/account/password", map[string]string{"currentPassword": creds["password"], "newPassword": "correcthorse99"}},
		{"/api/account/email", map[string]string{"currentPassword": "correcthorse99", "newEmail": "me2@example.com"}},
	} {
		b := newClient(t, base) // a second, possibly stolen, session
		login := map[string]string{"email": creds["email"], "password": creds["password"]}
		if step.path == "/api/account/email" {
			login["password"] = "correcthorse99"
		}
		b.do("POST", "/api/auth/login", login, false)
		if !signedIn(b) {
			t.Fatalf("%s: second session did not sign in", step.path)
		}
		if resp, _ := a.do("POST", step.path, step.body, true); resp.StatusCode != 200 {
			t.Fatalf("%s: %d", step.path, resp.StatusCode)
		}
		if signedIn(b) {
			t.Errorf("%s: other session still signed in", step.path)
		}
		if !signedIn(a) {
			t.Errorf("%s: the changing session was signed out", step.path)
		}
	}
}
