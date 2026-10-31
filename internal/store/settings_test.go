package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSeasonSwitchPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Season() != SeasonAuto {
		t.Fatalf("fresh store season = %q, want auto", st.Season())
	}
	for _, bad := range []string{"Christmas", "x y", "../etc", "a-very-long-season-id-that-goes-on-and-on"} {
		if st.SetSeason(bad, 1, time.Now()) == nil {
			t.Errorf("SetSeason(%q) accepted", bad)
		}
	}
	if err := st.SetSeason("christmas", 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if st, err = Open(path); err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if st.Season() != "christmas" {
		t.Fatalf("after reopen season = %q, want christmas", st.Season())
	}
	if err := st.SetSeason(SeasonOff, 1, time.Now()); err != nil || st.Season() != SeasonOff {
		t.Fatalf("off: %v %q", err, st.Season())
	}
}
