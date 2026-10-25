package api

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// rateLimiter is a small in-memory token-bucket limiter keyed by an arbitrary
// string (client IP, email, …). It bounds abuse of the unauthenticated
// email-sending endpoints (register, resend-verification) without any external
// dependency or shared state — good enough for a single-instance daemon.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens refilled per second
	burst   float64 // bucket capacity (max immediate requests)
}

type bucket struct {
	tokens float64
	last   time.Time
}

// newRateLimiter allows `burst` requests immediately, then one every
// 1/ratePerSec seconds. A background sweep evicts idle buckets so memory stays
// bounded under IP churn.
func newRateLimiter(ratePerSec, burst float64) *rateLimiter {
	rl := &rateLimiter{buckets: map[string]*bucket{}, rate: ratePerSec, burst: burst}
	go rl.sweepLoop()
	return rl
}

// Allow reports whether the request for key may proceed, consuming one token.
func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	b := rl.buckets[key]
	if b == nil {
		b = &bucket{tokens: rl.burst, last: now}
		rl.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * rl.rate
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (rl *rateLimiter) sweepLoop() {
	t := time.NewTicker(10 * time.Minute)
	for range t.C {
		rl.mu.Lock()
		now := time.Now()
		for k, b := range rl.buckets {
			// A bucket idle long enough to have fully refilled carries no state.
			if now.Sub(b.last) > 30*time.Minute {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

// emailRateOK reports whether an email-triggering request from this IP for this
// target address is within limits (IP bucket, then address bucket). Callers
// return 429 on false. It's checked before the work runs, so enumeration probes
// (e.g. register with a taken email) also consume the IP token — and because the
// 429 is returned regardless of whether the address exists, it leaks nothing.
func (s *Server) emailRateOK(r *http.Request, email string) bool {
	if !s.emailIPLimiter.Allow(clientIP(r)) {
		return false
	}
	if email != "" && !s.emailAddrLimiter.Allow(strings.ToLower(strings.TrimSpace(email))) {
		return false
	}
	return true
}

// clientIP returns the address rate limits are keyed on.
//
// Forwarding headers are believed only from a proxy hop: a loopback or private
// peer, which is what the deployments put in front of the daemon (cloudflared
// on the same host for dev; cloudflared and Caddy on the compose network for
// prod). From such a peer, Cloudflare's CF-Connecting-IP names the visitor.
// Failing that, the right-most PUBLIC entry of X-Forwarded-For is the one the
// nearest proxy appended; everything to its left was supplied by the client
// and is ignored. The left-most entry this used to take is whatever the
// client typed, which let anyone bypass the limits, or (behind a proxy that
// replaces the header) put every visitor in one shared bucket.
func clientIP(r *http.Request) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		peer = host
	}
	if !proxyHop(peer) {
		return peer
	}
	if cf := net.ParseIP(strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))); cf != nil {
		return cf.String()
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := net.ParseIP(strings.TrimSpace(parts[i]))
			if ip == nil {
				break // malformed: stop rather than read further into client input
			}
			if !proxyHop(ip.String()) {
				return ip.String()
			}
		}
	}
	return peer
}

// proxyHop reports whether addr is a loopback or private address, i.e. one of
// our own proxies rather than a visitor.
func proxyHop(addr string) bool {
	ip := net.ParseIP(addr)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

// sameOriginWS is the WebSocket upgrade origin check: allow non-browser clients
// (no Origin header) and requests whose Origin host matches the request Host;
// reject cross-site origins (prevents cross-site WebSocket hijacking).
func sameOriginWS(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // non-browser client (observer tooling, curl) — no Origin
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}
