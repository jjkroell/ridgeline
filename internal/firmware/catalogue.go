// Package firmware builds MeshCore images on request.
//
// The catalogue is derived from a checked-out MeshCore tree rather than hardcoded:
// upstream carries close to six hundred PlatformIO environments across sixty-odd
// boards, and any list maintained here would be wrong within a release. Reading
// the tree also means an environment the API offers is one that provably exists.
package firmware

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Option is a build choice a caller may ask for.
//
// The set is fixed here, server side, and each option expands to flags WE wrote.
// Nothing a client sends ever reaches the compiler as a flag: the request names
// an option id, and this table decides what that means. A build service that
// passes user text through to -D is a remote code execution waiting to happen.
type Option struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Help  string   `json:"help"`
	Flags []string `json:"-"`

	// requires, when set, limits the option to environments whose platformio.ini
	// section contains this marker. Offering an option that the chosen firmware
	// cannot honour produces a build indistinguishable from one without it.
	requires string
}

// Options is the entire allowlist.
var Options = []Option{
	{
		ID:    "packet-logging",
		Label: "Packet logging",
		Help:  "Log every received packet over serial. Needed for a node acting as a Ridgeline observer.",
		Flags: []string{"-D MESH_PACKET_LOGGING=1"},
	},
	{
		ID:    "buzzer-off",
		Label: "No buzzer",
		Help:  "Compile the buzzer out entirely. Nothing can beep — no startup chime, no message alert.",
		// Undefining the pin removes the buzzer class, every call site and all the
		// melodies: the #ifdef PIN_BUZZER guards go false. This replaced an earlier
		// attempt that only changed the SAVED PREFERENCE's factory default, which
		// was weaker in two ways — a device with existing settings ignored it
		// entirely, and upstream has reports of beeps escaping the mute anyway
		// (meshcore-dev#2233). Verified by the melody strings being absent from
		// the built image.
		//
		// No space after -U: PlatformIO's flag parser mangles the spaced form and
		// the compiler rejects it with "macro names must be identifiers".
		Flags:    []string{"-UPIN_BUZZER", "-UPIN_BUZZER_EN"},
		requires: "PIN_BUZZER",
	},
}

// Env is one buildable firmware.
type Env struct {
	Name     string   `json:"name"`
	Board    string   `json:"board"`
	Options  []string `json:"options"` // option ids valid for this env
	IsBridge bool     `json:"isBridge"`
}

// Board groups the environments of one hardware variant.
type Board struct {
	Name string `json:"name"`
	Envs []Env  `json:"envs"`
}

var envHeader = regexp.MustCompile(`(?m)^\[env:([^\]]+)\]`)

// LoadCatalogue reads every variant's platformio.ini under src.
func LoadCatalogue(src string) ([]Board, error) {
	dirs, err := os.ReadDir(filepath.Join(src, "variants"))
	if err != nil {
		return nil, fmt.Errorf("firmware: read variants: %w", err)
	}
	var boards []Board
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		ini := filepath.Join(src, "variants", d.Name(), "platformio.ini")
		raw, err := os.ReadFile(ini)
		if err != nil {
			continue // a variant directory without an ini has nothing to offer
		}
		envs := parseEnvs(d.Name(), string(raw))
		if len(envs) > 0 {
			boards = append(boards, Board{Name: d.Name(), Envs: envs})
		}
	}
	sort.Slice(boards, func(i, j int) bool { return boards[i].Name < boards[j].Name })
	return boards, nil
}

// parseEnvs splits an ini into its [env:...] sections and reads each one's
// capabilities from the text of that section alone.
func parseEnvs(board, ini string) []Env {
	locs := envHeader.FindAllStringSubmatchIndex(ini, -1)
	out := make([]Env, 0, len(locs))
	for i, loc := range locs {
		name := ini[loc[2]:loc[3]]
		end := len(ini)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		section := ini[loc[0]:end]

		e := Env{Name: name, Board: board, Options: []string{}}
		// A bridge build is a distinct upstream environment, never a flag we can
		// add to a plain repeater — surfacing it lets the UI say so.
		e.IsBridge = strings.Contains(section, "WITH_RS232_BRIDGE")
		for _, o := range Options {
			if o.requires == "" || strings.Contains(section, o.requires) {
				e.Options = append(e.Options, o.ID)
			}
		}
		out = append(out, e)
	}
	return out
}

// ResolveFlags turns requested option ids into compiler flags, rejecting any id
// that is unknown or not valid for this environment.
func ResolveFlags(boards []Board, envName string, optionIDs []string) ([]string, error) {
	var env *Env
	for _, b := range boards {
		for i := range b.Envs {
			if b.Envs[i].Name == envName {
				env = &b.Envs[i]
				break
			}
		}
	}
	if env == nil {
		return nil, fmt.Errorf("unknown environment %q", envName)
	}
	allowed := map[string]bool{}
	for _, id := range env.Options {
		allowed[id] = true
	}
	var flags []string
	for _, id := range optionIDs {
		if !allowed[id] {
			return nil, fmt.Errorf("option %q is not available for %s", id, envName)
		}
		for _, o := range Options {
			if o.ID == id {
				flags = append(flags, o.Flags...)
			}
		}
	}
	return flags, nil
}

// OptionIDsForFlags names the options a stored flag string represents.
//
// The reverse of ResolveFlags, for DISPLAY only: a finished job records compiler
// flags, and a list of "-D MESH_PACKET_LOGGING=1" tells a reader far less than
// "Packet logging". Both directions read the same Options table, so they cannot
// drift apart; anything unrecognised (an option since removed, say) is dropped
// rather than shown as a raw flag.
func OptionIDsForFlags(flags string) []string {
	if flags == "" {
		return []string{}
	}
	// Match on whole TOKENS, never substrings. "-UPIN_BUZZER" is a prefix of
	// "-UPIN_BUZZER_EN", so a Contains check reports the first option present
	// whenever only the second is — the kind of false positive that mislabels a
	// download and sends someone the wrong firmware.
	have := map[string]bool{}
	for _, tok := range strings.Fields(flags) {
		have[tok] = true
	}
	out := []string{}
	for _, o := range Options {
		if len(o.Flags) == 0 {
			continue
		}
		all := true
		for _, f := range o.Flags {
			// An option's flag may itself be several tokens ("-D NAME=1"); every one
			// must be present.
			for _, tok := range strings.Fields(f) {
				if !have[tok] {
					all = false
					break
				}
			}
			if !all {
				break
			}
		}
		if all {
			out = append(out, o.ID)
		}
	}
	return out
}
