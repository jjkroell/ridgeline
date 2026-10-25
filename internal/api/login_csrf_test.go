package api

import (
	"net/http"
	"strings"
	"testing"
)

// A cross-site page must not be able to sign a visitor in (login CSRF): the
// pre-session auth endpoints refuse cross-origin browser requests and any body
// that is not declared JSON, while same-origin and non-browser JSON still work.
func TestAuthEndpointsRefuseCrossSite(t *testing.T) {
	_, base, cleanup := newAuthEnv(t)
	defer cleanup()
	newClient(t, base).do("POST", "/api/auth/register",
		map[string]string{"email": "me@example.com", "password": "hunter2hunter2"}, false)

	login := func(ct string, hdr map[string]string) int {
		t.Helper()
		req, _ := http.NewRequest("POST", base+"/api/auth/login",
			strings.NewReader(`{"email":"me@example.com","password":"hunter2hunter2"}`))
		req.Header.Set("Content-Type", ct)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for name, c := range map[string]struct {
		ct   string
		hdr  map[string]string
		want int
	}{
		"cross-site fetch": {"text/plain", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		"foreign Origin":   {"application/json", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		"text/plain body":  {"text/plain", nil, http.StatusUnsupportedMediaType},
		"form body":        {"application/x-www-form-urlencoded", nil, http.StatusUnsupportedMediaType},
		"same-origin JSON": {"application/json", map[string]string{"Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		"non-browser JSON": {"application/json; charset=utf-8", nil, http.StatusOK},
	} {
		if got := login(c.ct, c.hdr); got != c.want {
			t.Errorf("%s: status %d, want %d", name, got, c.want)
		}
	}
}
