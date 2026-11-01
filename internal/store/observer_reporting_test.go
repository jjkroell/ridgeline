package store

import (
	"testing"
	"time"
)

func TestObserverReporting(t *testing.T) {
	now := time.Date(2026, 11, 1, 19, 4, 30, 0, time.UTC)
	cases := []struct {
		name     string
		lastSeen string
		want     bool
	}{
		{"seconds ago, nanosecond precision", "2026-11-01T19:04:28.838232779Z", true},
		{"just inside the window", now.Add(-ObserverFreshWindow + time.Second).Format(time.RFC3339Nano), true},
		{"just outside the window", now.Add(-ObserverFreshWindow - time.Second).Format(time.RFC3339Nano), false},
		{"an hour ago", now.Add(-time.Hour).Format(time.RFC3339), false},
		{"offset zone", "2026-11-01T11:04:00-08:00", true},
		{"empty", "", false},
		{"garbage", "yesterday", false},
	}
	for _, c := range cases {
		if got := observerReporting(c.lastSeen, now); got != c.want {
			t.Errorf("%s: observerReporting(%q) = %v, want %v", c.name, c.lastSeen, got, c.want)
		}
	}
}
