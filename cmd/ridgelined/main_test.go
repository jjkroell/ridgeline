package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/jjkroell/ridgeline/internal/config"
)

// A config that exists but does not load must stop the daemon. Falling back to
// defaults swaps in a different database path and a different broker, so the
// process comes up "healthy" while ingesting nothing and writing elsewhere —
// which is exactly how a bad subscriber entry took prod down while the only
// clue was a WARN line.
func TestFatalConfigError(t *testing.T) {
	missing := &fs.PathError{Op: "open", Path: "nope.json", Err: fs.ErrNotExist}
	cases := []struct {
		name     string
		err      error
		required bool
		want     bool
	}{
		{"absent file, not requested by name", missing, false, false},
		{"absent file, named with -config", missing, true, true},
		{"unreadable or malformed", os.ErrPermission, false, true},
	}
	for _, tc := range cases {
		if got := fatalConfigError(tc.err, tc.required); got != tc.want {
			t.Errorf("%s: fatalConfigError = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The two shapes of broken config that reach Load: unparseable, and parseable
// but rejected by validation. Both must be fatal; neither is fs.ErrNotExist.
func TestBrokenConfigIsFatal(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ name, body string }{
		{"malformed JSON", `{"listenAddr": ":8080",`},
		{"subscriber with an empty password", `{"mqttAuth":{"subscribers":[{"username":"dev","password":"","topics":["meshcore/#"]}]}}`},
	} {
		p := filepath.Join(dir, tc.name+".json")
		if err := os.WriteFile(p, []byte(tc.body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := config.Load(p)
		if err == nil {
			t.Fatalf("%s: Load succeeded, want an error", tc.name)
		}
		if !fatalConfigError(err, false) {
			t.Errorf("%s: would have fallen back to defaults", tc.name)
		}
	}
}

// The case defaults exist for: a checkout with nothing set up yet.
func TestAbsentConfigStillFallsBack(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "absent.json"))
	if err == nil {
		t.Fatal("Load of an absent file succeeded, want an error")
	}
	if fatalConfigError(err, false) {
		t.Error("an absent config should still fall back to defaults")
	}
	if !fatalConfigError(err, true) {
		t.Error("an absent config named with -config should be fatal")
	}
	_ = config.Default()
}
