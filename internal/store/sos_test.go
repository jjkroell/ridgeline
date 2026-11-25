package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
)

// Only accounts that switch it on are listed; a radio "join" isn't.
func TestSOSMonitors(t *testing.T) {
	RegisterSOSChannel()
	st := testStore(t)
	a, b := monitorUser(t, st, "a@x.ca"), monitorUser(t, st, "b@x.ca")
	names := func() string {
		t.Helper()
		var mem []string
		for _, m := range st.SOSMonitors() {
			if m.Since == "" {
				t.Errorf("%s has no since", m.Name)
			}
			mem = append(mem, m.Name)
		}
		return strings.Join(mem, ",")
	}
	set := func(id int64, on bool, name string) {
		t.Helper()
		if err := st.SetSOSMonitor(id, on, name); err != nil {
			t.Fatal(err)
		}
	}

	recordRaw(t, st, groupTextRaw(t, meshcore.DeriveHashtagKey(SOSChannel), "Radio", "join"), time.Now().UTC())
	if st.SOSMonitors() == nil || names() != "" {
		t.Fatalf("start = %q, want empty and not nil", names())
	}
	set(a, true, "zed")
	set(b, true, "Alice")
	if names() != "Alice,zed" {
		t.Fatalf("listed = %q", names())
	}
	// Someone else's listed name is refused, in any case; one's own is not.
	if err := st.SetSOSMonitor(b, true, "ZED"); err != ErrSOSNameTaken {
		t.Fatalf("taken name: %v", err)
	}
	since := st.SOSMonitors()[1].Since
	set(a, true, "Zed 2")
	if names() != "Alice,Zed 2" || st.SOSMonitors()[1].Since != since {
		t.Fatalf("renamed = %q", names())
	}
	// KOD-BOT leads, whatever sorts before it.
	bot := monitorUser(t, st, "bot@x.ca")
	set(bot, true, "KOD-BOT")
	if names() != "KOD-BOT,Alice,Zed 2" {
		t.Fatalf("pinned = %q", names())
	}
	set(bot, false, "")
	// Off: gone, name kept, and free for another account.
	set(a, false, "")
	if u, _, _ := st.GetUserByID(a); u.SOSMonitor || u.SOSMonitorName != "Zed 2" || names() != "Alice" {
		t.Fatalf("off: %+v, listed %q", u, names())
	}
	set(b, true, "zed 2")

	// The roster survives a reopen; blocking and deleting take accounts off it.
	if err := st.loadSOSMonitors(); err != nil || names() != "zed 2" {
		t.Fatalf("reload = %q, %v", names(), err)
	}
	if err := st.SetUserBlocked(b, true); err != nil || names() != "" {
		t.Fatalf("blocked = %q, %v", names(), err)
	}
	if err := st.SetUserBlocked(b, false); err != nil || names() != "zed 2" {
		t.Fatalf("unblocked = %q, %v", names(), err)
	}
	if err := st.DeleteUserAndReleaseNodes(b, "x"); err != nil || names() != "" {
		t.Fatalf("deleted = %q, %v", names(), err)
	}
}

// Only a heard companion's name counts; spaces and emoji may be left out when
// that isn't ambiguous.
func TestFindCompanion(t *testing.T) {
	st := testStore(t)
	for i, n := range []struct{ role, name, retired string }{
		{"ChatNode", "VE7KIP🌸", ""}, {"ChatNode", "KOD - Jesse", ""}, {"ChatNode", "Twin 📡", ""},
		{"ChatNode", "Twin 🗼", ""}, {"ChatNode", "🦅", ""}, {"ChatNode", "Gone", "2026-01-01T00:00:00Z"},
		{"ChatNode", "Blocked", ""}, {"Repeater", "Hilltop", ""}, {"ChatNode", "", ""},
	} {
		if _, err := st.db.Exec(`INSERT INTO nodes (pubkey, name, role, first_seen, last_seen, retired_at)
			VALUES (?,?,?,'','',?)`, fmt.Sprintf("%064X", i), n.name, n.role, n.retired); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.AddBlock(BlockNode, fmt.Sprintf("%064X", 6), "Blocked", ""); err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{
		"VE7KIP🌸": "VE7KIP🌸", " ve7kip ": "VE7KIP🌸", "kod jesse": "KOD - Jesse", "KOD": "", "🦅": "🦅",
		"Twin 🗼": "Twin 🗼", "twin": "", "Gone": "", "Blocked": "", "Hilltop": "", "": "", "📻": "",
	} {
		if got, ok, err := st.FindCompanion(in); err != nil || got != want || ok != (want != "") {
			t.Errorf("FindCompanion(%q) = %q, %v, %v; want %q", in, got, ok, err, want)
		}
	}
}

func monitorUser(t *testing.T, st *Store, email string) int64 {
	t.Helper()
	u, err := st.CreateUser(email, "h", "")
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

// Same cases as the web client's ALERT_RE; the two must agree.
func TestSOSAlertWords(t *testing.T) {
	for msg, want := range map[string]bool{
		"SOS at Mt Seymour":       true,
		"emergency!":              true,
		"Need help near Squamish": true,
		"he needs help":           true,
		"help me":                 true,
		"call 911":                true,
		"Mayday":                  true,
		"sos":                     true,
		"threat on the trail":     true,
		"life-threatening injury": true,
		"Life or death":           true,
		"threats":                 false,
		"lifeline":                false,
		"wildlife":                false,
		"announce #sos on Public": false,
		"thanks for the help":     false,
		"sosa":                    false,
		"test":                    false,
		// Replies (opening with an @-mention) never alert; a mention later does.
		"@[Dave] what's your emergency?":   false,
		" @[Dave] got your SOS, on my way": false,
		"SOS at the lake, @[Mike] is hurt": true,
	} {
		if got := sosAlertText(msg); got != want {
			t.Errorf("alert(%q) = %v, want %v", msg, got, want)
		}
	}
}

// The newest #sos alert is noted live and found again after a restart, and
// only within the window.
func TestSOSLastAlert(t *testing.T) {
	RegisterSOSChannel()
	st := testStore(t)
	key := meshcore.DeriveHashtagKey(SOSChannel)
	now := time.Now().UTC()

	recordRaw(t, st, groupTextRaw(t, key, "Hiker", "SOS rolled ankle"), now.Add(-3*time.Hour))
	recordRaw(t, st, groupTextRaw(t, key, "Hiker", "on my way"), now.Add(-time.Hour))
	want := now.Add(-3 * time.Hour).Format(time.RFC3339Nano)
	if got := st.SOSLastAlert(); got != want {
		t.Fatalf("live lastAlert = %q, want %q", got, want)
	}

	st.sosLastAlert = "" // as after a restart
	if err := st.LoadSOSAlert(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if got := st.SOSLastAlert(); got != want {
		t.Fatalf("loaded lastAlert = %q, want %q", got, want)
	}

	st.sosLastAlert = ""
	if err := st.LoadSOSAlert(context.Background(), now.Add(13*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := st.SOSLastAlert(); got != "" {
		t.Fatalf("alert outside the window loaded: %q", got)
	}
}

// The maps ring the node of whoever sent a #sos alert in the last 12 hours:
// newest per sender, newest first, survives a restart, ages out of the window.
func TestSOSAlertSenders(t *testing.T) {
	RegisterSOSChannel()
	st := testStore(t)
	key := meshcore.DeriveHashtagKey(SOSChannel)
	now := time.Now().UTC()
	at := func(d time.Duration) string { return now.Add(-d).Format(time.RFC3339Nano) }

	recordRaw(t, st, groupTextRaw(t, key, "Hiker", "SOS rolled ankle"), now.Add(-5*time.Hour))
	recordRaw(t, st, groupTextRaw(t, key, "Paddler", "mayday off Race Rocks"), now.Add(-2*time.Hour))
	recordRaw(t, st, groupTextRaw(t, key, "Hiker", "still need help"), now.Add(-time.Hour))
	recordRaw(t, st, groupTextRaw(t, key, "Helper", "@[Hiker] SOS team on the way"), now.Add(-30*time.Minute))
	recordRaw(t, st, groupTextRaw(t, key, "Chatty", "anyone around"), now.Add(-20*time.Minute))
	want := []SOSAlertSender{{"Hiker", at(time.Hour)}, {"Paddler", at(2 * time.Hour)}}
	check := func(label string, got []SOSAlertSender) {
		t.Helper()
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("%s: %+v, want %+v", label, got, want)
		}
	}
	check("live", st.SOSAlertSenders(now))

	st.sosAlerts = nil // as after a restart
	if err := st.LoadSOSAlert(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	check("loaded", st.SOSAlertSenders(now))

	if got := st.SOSAlertSenders(now.Add(10*time.Hour + 30*time.Minute)); len(got) != 1 || got[0].Sender != "Hiker" {
		t.Fatalf("after 10.5 h: %+v", got)
	}
	if got := st.SOSAlertSenders(now.Add(11*time.Hour + 30*time.Minute)); len(got) != 0 {
		t.Fatalf("after 11.5 h: %+v", got)
	}
}

// An admin's "resolved" clears a sender's ring for everyone, survives a
// restart, and a later alert from them rings again.
func TestSOSResolve(t *testing.T) {
	RegisterSOSChannel()
	st := testStore(t)
	key := meshcore.DeriveHashtagKey(SOSChannel)
	now := time.Now().UTC()
	recordRaw(t, st, groupTextRaw(t, key, "Hiker", "SOS rolled ankle"), now.Add(-time.Hour))
	recordRaw(t, st, groupTextRaw(t, key, "Paddler", "mayday"), now.Add(-time.Hour))

	if err := st.ResolveSOSAlert("Hiker", 1, now); err != nil {
		t.Fatal(err)
	}
	names := func() []string {
		var out []string
		for _, a := range st.SOSAlertSenders(time.Now()) {
			out = append(out, a.Sender)
		}
		return out
	}
	if got := names(); len(got) != 1 || got[0] != "Paddler" {
		t.Fatalf("after resolve: %v", got)
	}
	st.sosResolved = nil
	if err := st.loadSOSResolved(); err != nil {
		t.Fatal(err)
	}
	if got := names(); len(got) != 1 || got[0] != "Paddler" {
		t.Fatalf("after reload: %v", got)
	}
	recordRaw(t, st, groupTextRaw(t, key, "Hiker", "SOS again, worse"), now.Add(time.Minute))
	if got := names(); len(got) != 2 || got[0] != "Hiker" {
		t.Fatalf("new alert should ring again: %v", got)
	}
}

// Resolving every open alert stops the channel pulse (lastAlert) for
// everyone; one still open keeps it going; a new alert starts it again.
func TestSOSResolveStopsPulse(t *testing.T) {
	RegisterSOSChannel()
	st := testStore(t)
	key := meshcore.DeriveHashtagKey(SOSChannel)
	now := time.Now().UTC()
	at := func(d time.Duration) string { return now.Add(d).Format(time.RFC3339Nano) }
	recordRaw(t, st, groupTextRaw(t, key, "Paddler", "mayday"), now.Add(-2*time.Hour))
	recordRaw(t, st, groupTextRaw(t, key, "Hiker", "SOS rolled ankle"), now.Add(-time.Hour))

	if err := st.ResolveSOSAlert("Hiker", 1, now); err != nil {
		t.Fatal(err)
	}
	if got, want := st.SOSLastAlert(), at(-2*time.Hour); got != want {
		t.Fatalf("one still open: lastAlert = %q, want the Paddler's %q", got, want)
	}
	if got, want := st.SOSAlertsClearedAt(), at(-time.Hour); got != want {
		t.Fatalf("clearedAt = %q, want %q", got, want)
	}
	if err := st.ResolveSOSAlert("Paddler", 1, now); err != nil {
		t.Fatal(err)
	}
	if got := st.SOSLastAlert(); got != "" {
		t.Fatalf("all resolved: lastAlert = %q, want none", got)
	}
	recordRaw(t, st, groupTextRaw(t, key, "Hiker", "SOS again"), now.Add(time.Minute))
	if got, want := st.SOSLastAlert(), at(time.Minute); got != want {
		t.Fatalf("new alert: lastAlert = %q, want %q", got, want)
	}
}

// Purging a channel removes its observations (every observer's copy) and
// nothing else.
func TestPurgeChannel(t *testing.T) {
	RegisterSOSChannel()
	st := testStore(t)
	key := meshcore.DeriveHashtagKey(SOSChannel)
	pub := []byte{0x8b, 0x33, 0x87, 0xe9, 0xc5, 0xcd, 0xea, 0x6a, 0xc9, 0xe5, 0xed, 0xba, 0xa1, 0x15, 0xcd, 0x72}
	now := time.Now().UTC()

	test := groupTextRaw(t, key, "Gail", "join")
	recordRaw(t, st, test, now.Add(-time.Hour))
	recordRaw(t, st, test, now.Add(-time.Hour+time.Second)) // second observer
	recordRaw(t, st, groupTextRaw(t, key, "Gail", "anyone?"), now.Add(-30*time.Minute))
	recordRaw(t, st, groupTextRaw(t, pub, "Hal", "hello"), now.Add(-30*time.Minute))

	obs, err := st.ChannelObservations(context.Background(), SOSChannel, 1, now)
	if err != nil || len(obs) != 3 {
		t.Fatalf("found %d #sos observations (%v), want 3", len(obs), err)
	}
	ids := make([]int64, len(obs))
	for i, o := range obs {
		ids[i] = o.ID
	}
	if n, err := st.DeleteObservations(ids); err != nil || n != 3 {
		t.Fatalf("deleted %d (%v), want 3", n, err)
	}
	if left, _ := st.ChannelObservations(context.Background(), SOSChannel, 1, now); len(left) != 0 {
		t.Fatalf("%d #sos observations left", len(left))
	}
	var total int
	st.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&total)
	if total != 1 {
		t.Fatalf("%d observations left, want the 1 Public message", total)
	}
}
