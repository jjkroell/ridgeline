package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterBurstThenDenyThenRefill(t *testing.T) {
	rl := newRateLimiter(1000, 3) // 3 burst, fast refill

	for i := 0; i < 3; i++ {
		if !rl.Allow("k") {
			t.Fatalf("burst token %d should be allowed", i+1)
		}
	}
	if rl.Allow("k") {
		t.Fatal("4th immediate request should be denied (burst exhausted)")
	}
	// A different key has its own bucket.
	if !rl.Allow("other") {
		t.Fatal("an independent key should not be affected")
	}
	// After a short wait the bucket refills (1000 tok/s -> ~5 tokens in 5ms).
	time.Sleep(5 * time.Millisecond)
	if !rl.Allow("k") {
		t.Fatal("bucket should refill after waiting")
	}
}

// The rate-limit key trusts forwarding headers only from a proxy hop, and
// never takes a client-supplied (left-most) X-Forwarded-For entry.
func TestClientIP(t *testing.T) {
	cases := []struct {
		name, remote, cf, xff, want string
	}{
		{"proxy peer, no headers", "10.0.0.5:1234", "", "", "10.0.0.5"},
		{"tunnel names the visitor", "127.0.0.1:5555", "198.51.100.7", "1.2.3.4", "198.51.100.7"},
		{"spoofed left-most XFF ignored", "172.18.0.3:80", "", "203.0.113.9, 70.1.2.3", "70.1.2.3"},
		{"private hops skipped from the right", "172.18.0.3:80", "", "203.0.113.9, 70.1.2.3, 172.18.0.2", "70.1.2.3"},
		{"direct visitor cannot set headers", "70.1.2.3:4444", "198.51.100.7", "203.0.113.9", "70.1.2.3"},
		{"bad CF header falls back to XFF", "127.0.0.1:1", "not-an-ip", "70.1.2.3", "70.1.2.3"},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = c.remote
		if c.cf != "" {
			r.Header.Set("CF-Connecting-IP", c.cf)
		}
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		if got := clientIP(r); got != c.want {
			t.Errorf("%s: clientIP = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSameOriginWS(t *testing.T) {
	mk := func(origin, host string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/live", nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}
	if !sameOriginWS(mk("", "mesh.example")) {
		t.Error("no Origin (non-browser client) should be allowed")
	}
	if !sameOriginWS(mk("https://mesh.example", "mesh.example")) {
		t.Error("same-origin request should be allowed")
	}
	if sameOriginWS(mk("https://evil.example", "mesh.example")) {
		t.Error("cross-origin request should be denied")
	}
}
