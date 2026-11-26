package api

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// sosPacket builds a flood GroupText packet on #sos from sender with msg.
func sosPacket(t *testing.T, sender, msg string) (*meshcore.Packet, string) {
	t.Helper()
	key := meshcore.DeriveHashtagKey(store.SOSChannel)
	body := make([]byte, 5)
	binary.LittleEndian.PutUint32(body[0:4], uint32(time.Now().Unix()))
	body = append(body, []byte(sender+": "+msg)...)
	for len(body)%aes.BlockSize != 0 {
		body = append(body, 0)
	}
	block, _ := aes.NewCipher(key)
	ct := make([]byte, len(body))
	for i := 0; i < len(body); i += aes.BlockSize {
		block.Encrypt(ct[i:i+aes.BlockSize], body[i:i+aes.BlockSize])
	}
	secret := make([]byte, 32)
	copy(secret, key)
	m := hmac.New(sha256.New, secret)
	m.Write(ct)
	sum := m.Sum(nil)
	h := sha256.Sum256(key)
	raw := hex.EncodeToString(append([]byte{0x15, 0x00, h[0], sum[0], sum[1]}, ct...))
	p, err := meshcore.DecodeHex(raw)
	if err != nil {
		t.Fatal(err)
	}
	return p, raw
}

// Opted-in, verified accounts get one batched email per burst of #sos alerts;
// repeat copies, non-alert messages and opted-out accounts get nothing.
func TestSOSAlertEmails(t *testing.T) {
	store.RegisterSOSChannel()
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.SetEmergencyEnabled(true, 0, time.Now())
	srv := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "")
	fm := &fakeMailer{base: "https://ridgeline.example"}
	srv.SetMailer(fm)
	srv.sosMail.coalesce, srv.sosMail.minGap = 50*time.Millisecond, 300*time.Millisecond
	st.SetSOSAlertHook(srv.NotifySOSAlert)

	in, _ := st.CreateUser("in@example.com", "x", "Watcher") // first user: auto-verified owner
	out, _ := st.CreateUser("out@example.com", "x", "Quiet")
	st.MarkEmailVerified(out.ID)
	unverified, _ := st.CreateUser("unv@example.com", "x", "New")
	st.SetSOSAlerts(in.ID, true)
	st.SetSOSAlerts(unverified.ID, true) // opted in but never verified: skipped
	_ = out

	record := func(p *meshcore.Packet, raw string, observer string) {
		t.Helper()
		if err := st.Record(store.Observation{Packet: p, RawHex: raw, ObserverID: observer, ReceivedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	p1, r1 := sosPacket(t, "Hiker", "SOS rolled ankle <b>summit</b>")
	record(p1, r1, "obs-a")
	record(p1, r1, "obs-b") // another observer's copy: no second alert
	p2, r2 := sosPacket(t, "Hiker", "need help, cold")
	record(p2, r2, "obs-a") // joins the same batch
	p3, r3 := sosPacket(t, "Chatty", "anyone up?")
	record(p3, r3, "obs-a") // not an alert

	time.Sleep(200 * time.Millisecond)
	got := fm.ofKind("sos-alert")
	if len(got) != 1 || got[0].to != "in@example.com" {
		t.Fatalf("emails = %+v, want one to in@example.com", got)
	}
	m := got[0]
	if !strings.Contains(m.subject, "2 messages") {
		t.Errorf("subject %q should count the 2 batched alerts", m.subject)
	}
	for _, want := range []string{"SOS rolled ankle", "need help, cold", "/channels?channel=mayday", "/account#sos-alerts", "911"} {
		if !strings.Contains(m.text, want) {
			t.Errorf("text missing %q:\n%s", want, m.text)
		}
	}
	if strings.Contains(m.html, "<b>summit</b>") || !strings.Contains(m.html, "&lt;b&gt;summit") {
		t.Error("message text must be HTML-escaped in the email body")
	}
	if strings.Contains(m.text, "anyone up?") {
		t.Error("a non-alert message was mailed")
	}

	// Inside the minimum gap, the next alert waits rather than sending at once.
	p4, r4 := sosPacket(t, "Hiker", "mayday")
	record(p4, r4, "obs-a")
	time.Sleep(100 * time.Millisecond)
	if n := len(fm.ofKind("sos-alert")); n != 1 {
		t.Fatalf("sent %d emails inside the minimum gap, want still 1", n)
	}
	time.Sleep(400 * time.Millisecond)
	if got := fm.ofKind("sos-alert"); len(got) != 2 || !strings.HasSuffix(got[1].subject, "Hiker") {
		t.Fatalf("after the gap: %+v", got)
	}

	// Opting out stops them.
	st.SetSOSAlerts(in.ID, false)
	p5, r5 := sosPacket(t, "Hiker", "emergency")
	record(p5, r5, "obs-a")
	time.Sleep(500 * time.Millisecond)
	if n := len(fm.ofKind("sos-alert")); n != 2 {
		t.Fatalf("opted-out account still mailed: %d emails", n)
	}
}

// Alerts that say they're a test reach only accounts opted in to tests, and a
// batch of nothing but tests doesn't hold up the next real call.
func TestSOSAlertTestMessages(t *testing.T) {
	store.RegisterSOSChannel()
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.SetEmergencyEnabled(true, 0, time.Now())
	srv := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "")
	fm := &fakeMailer{base: "https://ridgeline.example"}
	srv.SetMailer(fm)
	srv.sosMail.coalesce, srv.sosMail.minGap = 50*time.Millisecond, 10*time.Second
	st.SetSOSAlertHook(srv.NotifySOSAlert)

	real, _ := st.CreateUser("real@example.com", "x", "Real") // first user: verified
	tester, _ := st.CreateUser("tests@example.com", "x", "Tester")
	st.MarkEmailVerified(tester.ID)
	st.SetSOSAlerts(real.ID, true)
	st.SetSOSAlerts(tester.ID, true)
	st.SetSOSAlertTests(tester.ID, true)
	if u, _, _ := st.GetUserByID(tester.ID); !u.SOSAlertTests {
		t.Fatal("sosAlertTests not stored")
	}

	send := func(msg string) {
		t.Helper()
		p, raw := sosPacket(t, "Jesse", msg)
		if err := st.Record(store.Observation{Packet: p, RawHex: raw, ObserverID: "obs", ReceivedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	to := func(addr string) (n int) {
		for _, m := range fm.ofKind("sos-alert") {
			if m.to == addr {
				n++
			}
		}
		return
	}

	send("Sos testing bot auto reply")
	time.Sleep(200 * time.Millisecond)
	if to("tests@example.com") != 1 || to("real@example.com") != 0 {
		t.Fatalf("test alert: tests=%d real=%d, want 1 and 0", to("tests@example.com"), to("real@example.com"))
	}

	// The tester's email started the gap, so the next waits; reset the gap to
	// check a tests-only batch on its own.
	srv.sosMail.mu.Lock()
	srv.sosMail.lastSent = time.Time{}
	srv.sosMail.mu.Unlock()
	st.SetSOSAlertTests(tester.ID, false)
	send("another SOS test")
	time.Sleep(200 * time.Millisecond)
	if n := len(fm.ofKind("sos-alert")); n != 1 {
		t.Fatalf("tests-only batch with no takers sent %d emails, want still 1", n)
	}
	send("SOS stuck on the ridge") // real: must not wait out a 10 s gap
	time.Sleep(200 * time.Millisecond)
	if to("real@example.com") != 1 || to("tests@example.com") != 2 {
		t.Fatalf("real alert after a tests-only batch: real=%d tests=%d", to("real@example.com"), to("tests@example.com"))
	}
}

// A real call right after a test email doesn't wait out the 10-minute gap,
// whether it arrives after the send or is already queued behind the gap; a
// real email still holds back the next one.
func TestSOSAlertRealSkipsGapAfterTest(t *testing.T) {
	store.RegisterSOSChannel()
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.SetEmergencyEnabled(true, 0, time.Now())
	srv := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "")
	fm := &fakeMailer{base: "https://ridgeline.example"}
	srv.SetMailer(fm)
	srv.sosMail.coalesce, srv.sosMail.minGap = 50*time.Millisecond, 10*time.Second
	st.SetSOSAlertHook(srv.NotifySOSAlert)

	u, _ := st.CreateUser("jesse@example.com", "x", "Jesse")
	st.SetSOSAlerts(u.ID, true)
	st.SetSOSAlertTests(u.ID, true)

	send := func(msg string) {
		t.Helper()
		p, raw := sosPacket(t, "Jesse", msg)
		if err := st.Record(store.Observation{Packet: p, RawHex: raw, ObserverID: "obs", ReceivedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	count := func() int { return len(fm.ofKind("sos-alert")) }

	send("SOS test of the system")
	time.Sleep(200 * time.Millisecond)
	if count() != 1 {
		t.Fatalf("test alert: %d emails, want 1", count())
	}
	send("SOS test again") // a test still waits out the gap…
	time.Sleep(200 * time.Millisecond)
	if count() != 1 {
		t.Fatalf("second test inside the gap: %d emails, want 1", count())
	}
	send("SOS hurt on the trail") // …but a real call pulls the queued send forward
	time.Sleep(200 * time.Millisecond)
	if count() != 2 {
		t.Fatalf("real alert queued behind a test gap: %d emails, want 2", count())
	}
	send("SOS second real call") // after a real email the gap holds
	time.Sleep(200 * time.Millisecond)
	if count() != 2 {
		t.Fatalf("real alert inside a real gap: %d emails, want still 2", count())
	}
}

// The "I'm monitoring #sos" switch: signed in only, with a radio that has been heard.
func TestSOSMonitorSwitch(t *testing.T) {
	st, base, fm, cleanup := mailEnv(t)
	defer cleanup()
	st.SetEmergencyEnabled(true, 0, time.Now())
	owner, member, anon := newClient(t, base), newClient(t, base), newClient(t, base)
	owner.do("POST", "/api/auth/register",
		map[string]string{"email": "owner@example.com", "password": "hunter2hunter2", "displayName": "Someone"}, false)
	member.do("POST", "/api/auth/register",
		map[string]string{"email": "member@example.com", "password": "hunter2hunter2"}, false)
	vs := fm.ofKind("verification")
	member.do("POST", "/api/auth/verify", map[string]string{"token": tokenFromLink(t, vs[len(vs)-1].text)}, false)

	put := func(c *authTestClient, body any, csrf bool, want int) map[string]any {
		t.Helper()
		r, got := c.do("PUT", "/api/account/sos-monitor", body, csrf)
		if r.StatusCode != want {
			t.Fatalf("%v: got %d %v, want %d", body, r.StatusCode, got, want)
		}
		return got
	}
	type m = map[string]any
	put(anon, m{"enabled": true, "name": "X"}, false, 401)
	put(owner, m{"enabled": true}, false, 403) // no CSRF token
	put(owner, m{"name": "X"}, true, 400)
	put(member, m{"enabled": true, "name": " "}, true, 400)
	e := put(member, m{"enabled": true, "name": "Nobody"}, true, 422)
	if msg, _ := e["error"].(string); !strings.Contains(msg, "flood advert") {
		t.Fatalf("unheard radio: %v", e)
	}
	seedCompanion(t, st, "VE7AAA")
	seedCompanion(t, st, "Bob Hill 📡")
	got := put(owner, m{"enabled": true, "name": " ve7aaa "}, true, 200)
	if got["sosMonitor"] != true || got["sosMonitorName"] != "VE7AAA" {
		t.Fatalf("owner = %v", got)
	}
	put(member, m{"enabled": true, "name": "VE7AAA"}, true, 409)
	put(member, m{"enabled": true, "name": "bob hill"}, true, 200) // the emoji needn't be typed
	mons := st.SOSMonitors()
	if len(mons) != 2 || mons[0].Name != "Bob Hill 📡" || mons[1].Name != "VE7AAA" {
		t.Fatalf("monitors = %+v", mons)
	}
	// What the public list carries: a name and a date, no account details.
	r, body := anon.do("GET", "/api/channels/sos", nil, false)
	list, _ := body["monitors"].([]any)
	if r.StatusCode != 200 || len(list) != 2 {
		t.Fatalf("roster = %d %v", r.StatusCode, body)
	}
	if e, _ := list[0].(map[string]any); len(e) != 2 || e["name"] != "Bob Hill 📡" || e["since"] == "" {
		t.Fatalf("entry = %v", list[0])
	}
	put(member, m{"enabled": false}, true, 200)
	if mons := st.SOSMonitors(); len(mons) != 1 {
		t.Fatalf("after off = %+v", mons)
	}

	// The admin switch: members can't use it; off empties the public roster
	// and drops "emergency" from health, keeping the stored list; on restores it.
	if r, _ := member.do("POST", "/api/admin/emergency", m{"enabled": false}, true); r.StatusCode != 403 {
		t.Fatalf("member switched it: %d", r.StatusCode)
	}
	if r, b := owner.do("POST", "/api/admin/emergency", m{"enabled": false}, true); r.StatusCode != 200 || b["enabled"] != false {
		t.Fatalf("owner off: %d %v", r.StatusCode, b)
	}
	_, body = anon.do("GET", "/api/channels/sos", nil, false)
	if list, _ := body["monitors"].([]any); body["enabled"] != false || len(list) != 0 || body["channel"] != "#mayday" {
		t.Fatalf("roster while off = %v", body)
	}
	if _, h := anon.do("GET", "/api/health", nil, false); h["emergency"] != nil {
		t.Fatalf("health while off = %v", h)
	}
	owner.do("POST", "/api/admin/emergency", m{"enabled": true}, true)
	_, body = anon.do("GET", "/api/channels/sos", nil, false)
	if list, _ := body["monitors"].([]any); body["enabled"] != true || len(list) != 1 {
		t.Fatalf("roster back on = %v", body)
	}
	if _, h := anon.do("GET", "/api/health", nil, false); h["emergency"] != "mayday" {
		t.Fatalf("health while on = %v", h)
	}
}

// seedCompanion records a signed advert from a companion radio called name.
func seedCompanion(t *testing.T, st *store.Store, name string) {
	t.Helper()
	h := sha256.Sum256([]byte(name))
	p := &meshcore.Packet{MessageHash: hex.EncodeToString(h[:8]), Advert: &meshcore.Advert{
		PublicKey: strings.ToUpper(hex.EncodeToString(h[:])), DeviceRole: meshcore.RoleChatNode,
		HasName: true, Name: name, SignatureValid: true}}
	if err := st.Record(store.Observation{Packet: p, RawHex: "00", ObserverID: "obs", ReceivedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
}
