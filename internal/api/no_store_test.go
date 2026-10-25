package api

import (
	"net/http"
	"testing"
)

// Signed-in API responses are no-store so the service worker never caches
// per-user data; anonymous ones stay cacheable for offline use.
func TestSignedInAPIResponsesAreNoStore(t *testing.T) {
	_, base, cleanup := newAuthEnv(t)
	defer cleanup()
	c := newClient(t, base)
	c.do("POST", "/api/auth/register", map[string]string{"email": "me@example.com", "password": "hunter2hunter2"}, false)

	resp, _ := c.do("GET", "/api/auth/me", nil, false)
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("signed-in /api/auth/me Cache-Control = %q, want no-store", got)
	}
	anon, err := http.Get(base + "/api/nodes")
	if err != nil {
		t.Fatal(err)
	}
	anon.Body.Close()
	if got := anon.Header.Get("Cache-Control"); got == "no-store" {
		t.Error("anonymous /api/nodes should stay cacheable")
	}
}
