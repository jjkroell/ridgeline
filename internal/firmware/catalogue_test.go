package firmware

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A miniature variants tree: one board whose companion env compiles a buzzer and
// whose repeater env does not, plus a bridge env.
func writeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "variants", "testboard")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ini := `[env:tb_repeater]
build_flags =
   -D SOMETHING=1

[env:tb_companion_ble]
build_flags =
   -D PIN_BUZZER=25
   -D PIN_BUZZER_EN=37

[env:tb_repeater_bridge_rs232]
build_flags =
   -D WITH_RS232_BRIDGE=Serial1
`
	if err := os.WriteFile(filepath.Join(dir, "platformio.ini"), []byte(ini), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCatalogueReadsEnvsAndCapabilities(t *testing.T) {
	boards, err := LoadCatalogue(writeTree(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 1 || boards[0].Name != "testboard" {
		t.Fatalf("boards = %+v, want one testboard", boards)
	}
	got := map[string]Env{}
	for _, e := range boards[0].Envs {
		got[e.Name] = e
	}
	if len(got) != 3 {
		t.Fatalf("envs = %d, want 3", len(got))
	}
	if !got["tb_repeater_bridge_rs232"].IsBridge {
		t.Error("bridge env not flagged as a bridge")
	}
	if got["tb_repeater"].IsBridge {
		t.Error("plain repeater flagged as a bridge")
	}
}

// The buzzer option must be offered ONLY where a buzzer is compiled in. Offering
// it elsewhere produces firmware identical to one built without it, which looks
// to the requester exactly like the option silently not working.
func TestBuzzerOptionOnlyOfferedWhereABuzzerExists(t *testing.T) {
	boards, err := LoadCatalogue(writeTree(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range boards[0].Envs {
		has := false
		for _, id := range e.Options {
			if id == "buzzer-off" {
				has = true
			}
		}
		want := e.Name == "tb_companion_ble"
		if has != want {
			t.Errorf("%s: buzzer-off offered = %v, want %v", e.Name, has, want)
		}
	}
}

// Packet logging applies to every environment, so it must be offered on all.
func TestPacketLoggingOfferedEverywhere(t *testing.T) {
	boards, err := LoadCatalogue(writeTree(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range boards[0].Envs {
		found := false
		for _, id := range e.Options {
			if id == "packet-logging" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: packet-logging not offered", e.Name)
		}
	}
}

func TestResolveFlags(t *testing.T) {
	boards, err := LoadCatalogue(writeTree(t))
	if err != nil {
		t.Fatal(err)
	}

	flags, err := ResolveFlags(boards, "tb_companion_ble", []string{"packet-logging", "buzzer-off"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(flags, " ")
	for _, want := range []string{"-D MESH_PACKET_LOGGING=1", "-UPIN_BUZZER"} {
		if !strings.Contains(joined, want) {
			t.Errorf("flags %q missing %q", joined, want)
		}
	}

	// An option the environment cannot honour is an error, never a silent drop.
	if _, err := ResolveFlags(boards, "tb_repeater", []string{"buzzer-off"}); err == nil {
		t.Error("buzzer-off accepted for an env with no buzzer")
	}
	// Anything not in the allowlist is refused — this is the boundary that keeps
	// caller-supplied text away from the compiler.
	if _, err := ResolveFlags(boards, "tb_repeater", []string{"-D EVIL=1"}); err == nil {
		t.Error("raw flag accepted as an option id")
	}
	if _, err := ResolveFlags(boards, "no_such_env", nil); err == nil {
		t.Error("unknown environment accepted")
	}
}

// "-UPIN_BUZZER" is a strict prefix of "-UPIN_BUZZER_EN". Matching on substrings
// would report buzzer-off whenever only the _EN flag was present, mislabelling a
// download as an option it was not built with.
func TestOptionIDsMatchWholeTokensNotSubstrings(t *testing.T) {
	if got := OptionIDsForFlags("-UPIN_BUZZER_EN"); len(got) != 0 {
		t.Errorf("OptionIDsForFlags(\"-UPIN_BUZZER_EN\") = %v, want none — the option needs BOTH flags", got)
	}
	got := OptionIDsForFlags("-UPIN_BUZZER -UPIN_BUZZER_EN")
	if len(got) != 1 || got[0] != "buzzer-off" {
		t.Errorf("OptionIDsForFlags(both) = %v, want [buzzer-off]", got)
	}
}
