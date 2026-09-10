package api

import "testing"

// The download handler takes a filename straight from the URL. It is checked
// against a fixed set of names rather than cleaned, so traversal has nothing to
// work with — this pins that property.
func TestArtifactNamesRejectTraversal(t *testing.T) {
	for _, bad := range []string{
		"../../../etc/passwd",
		"..%2f..%2fconfig.json",
		"/etc/passwd",
		"firmware.bin/../../secret",
		".",
		"",
		"config.json",
		"ridgeline.db",
	} {
		if artifactNames[bad] {
			t.Errorf("%q accepted as an artifact name", bad)
		}
	}
	for _, good := range []string{"firmware-merged.bin", "firmware.zip", "firmware.uf2", "firmware.hex"} {
		if !artifactNames[good] {
			t.Errorf("%q rejected but is a real artifact", good)
		}
	}
}

// A download must be identifiable once it is sitting in someone's downloads
// folder: which board, which release, and which options. The build directory
// calls everything firmware.*, so two builds of one board differing only by
// option would otherwise arrive under the same name.
func TestDownloadNameCarriesBoardVersionAndOptions(t *testing.T) {
	const env = "t1000e_companion_radio_ble"
	cases := []struct{ tag, flags, artifact, want string }{
		// The tag names the firmware LINE upstream, not what is being built here:
		// a companion build carries "repeater-v1.17.1", which reads as an error.
		{"repeater-v1.17.1", "", "firmware.uf2", "t1000e_companion_radio_ble-1.17.1.uf2"},
		{"repeater-v1.17.1", "-UPIN_BUZZER -UPIN_BUZZER_EN", "firmware.uf2",
			"t1000e_companion_radio_ble-1.17.1-buzzer-off.uf2"},
		{"companion-v1.17.1", "-D MESH_PACKET_LOGGING=1", "firmware.zip",
			"t1000e_companion_radio_ble-1.17.1-packet-logging.zip"},
		// ESP32 merged image keeps its role in the name.
		{"repeater-v1.17.1", "-D MESH_PACKET_LOGGING=1", "firmware-merged.bin",
			"t1000e_companion_radio_ble-1.17.1-merged-packet-logging.bin"},
		// Component images are identical across option sets, so they carry no suffix.
		{"repeater-v1.17.1", "-D MESH_PACKET_LOGGING=1", "bootloader.bin",
			"t1000e_companion_radio_ble-1.17.1-bootloader.bin"},
	}
	for _, c := range cases {
		if got := downloadName(env, c.tag, c.flags, c.artifact); got != c.want {
			t.Errorf("downloadName(%q, %q, %q) = %q, want %q", c.tag, c.flags, c.artifact, got, c.want)
		}
	}
}

// Two option sets must never produce the same filename, or the saved files are
// indistinguishable — the whole reason for this scheme.
func TestDownloadNamesAreDistinctPerOptionSet(t *testing.T) {
	seen := map[string]string{}
	for _, flags := range []string{
		"",
		"-UPIN_BUZZER -UPIN_BUZZER_EN",
		"-D MESH_PACKET_LOGGING=1",
		"-UPIN_BUZZER -UPIN_BUZZER_EN -D MESH_PACKET_LOGGING=1",
	} {
		n := downloadName("t1000e_companion_radio_ble", "repeater-v1.17.1", flags, "firmware.uf2")
		if prev, dup := seen[n]; dup {
			t.Errorf("flags %q and %q both produce %q", prev, flags, n)
		}
		seen[n] = flags
	}
}

// A tag with no version must still yield a usable name rather than an empty one.
func TestDownloadNameFallsBackWhenTagHasNoVersion(t *testing.T) {
	got := downloadName("some_env", "nightly", "", "firmware.uf2")
	if got != "some_env-nightly.uf2" {
		t.Errorf("downloadName = %q, want some_env-nightly.uf2", got)
	}
}
