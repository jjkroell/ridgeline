package api

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

// #sos alert emails. Accounts that opt in (Account → #sos alerts) are emailed
// when an emergency word is heard on #sos — the same words that pulse the
// channel in the UI (store.isSOSAlert). Anyone on the mesh can type "sos", so
// alerts are batched: the first waits sosAlertCoalesce for others to join it,
// and after a send the next waits until sosAlertMinGap has passed, carrying
// everything heard meanwhile. A burst of messages is a few emails, not dozens.
const (
	sosAlertCoalesce = 30 * time.Second
	sosAlertMinGap   = 10 * time.Minute
	sosAlertMaxShown = 20 // messages listed in one email; the rest are counted
)

// sosTestRe marks an alert that says it's a test ("SOS test of the auto
// reply"). Those go only to accounts that opted in to test messages.
var sosTestRe = regexp.MustCompile(`(?i)\btest(s|ing|ed)?\b`)

type sosNotifier struct {
	mu       sync.Mutex
	pending  []store.SOSAlertMsg
	timer    *time.Timer
	lastSent time.Time
	coalesce time.Duration
	minGap   time.Duration
}

// NotifySOSAlert queues a newly heard #sos alert for the opted-in accounts.
// The store's alert hook; never blocks (Record holds the write lock).
func (s *Server) NotifySOSAlert(m store.SOSAlertMsg) {
	if !s.mailEnabled() {
		return
	}
	n := &s.sosMail
	n.mu.Lock()
	defer n.mu.Unlock()
	n.pending = append(n.pending, m)
	if n.timer != nil {
		return // a send is already scheduled; this message rides along
	}
	wait := n.coalesce
	if gap := time.Until(n.lastSent.Add(n.minGap)); gap > wait {
		wait = gap
	}
	n.timer = time.AfterFunc(wait, s.flushSOSAlerts)
}

// flushSOSAlerts emails everything pending to every opted-in account.
func (s *Server) flushSOSAlerts() {
	n := &s.sosMail
	n.mu.Lock()
	batch := n.pending
	prevSent := n.lastSent
	n.pending, n.timer, n.lastSent = nil, nil, time.Now()
	n.mu.Unlock()
	if len(batch) == 0 {
		return
	}
	users, err := s.store.SOSAlertRecipients()
	if err != nil {
		s.log.Error("sos alert recipients", "err", err)
		return
	}
	if len(users) == 0 {
		return
	}
	// Accounts that didn't opt in to test messages get the batch without them,
	// or nothing if that's all it held.
	var real []store.SOSAlertMsg
	for _, m := range batch {
		if !sosTestRe.MatchString(m.Message) {
			real = append(real, m)
		}
	}
	var all, noTests *[3]string
	sent := 0
	for _, u := range users {
		msgs, cached := batch, &all
		if !u.SOSAlertTests {
			msgs, cached = real, &noTests
		}
		if len(msgs) == 0 {
			continue
		}
		if *cached == nil {
			subject, text, body := s.sosAlertEmail(msgs)
			*cached = &[3]string{subject, text, body}
		}
		e := *cached
		s.mail.SendAsync("sos-alert", u.Email, e[0], e[1], e[2])
		sent++
	}
	if sent == 0 {
		// Only test messages, and nobody wanted those: no email went out, so
		// a real call right after shouldn't wait out the gap.
		n.mu.Lock()
		n.lastSent = prevSent
		if n.timer != nil && n.timer.Stop() {
			// A message arrived meanwhile and was scheduled behind the gap.
			n.timer = time.AfterFunc(n.coalesce, s.flushSOSAlerts)
		}
		n.mu.Unlock()
		return
	}
	s.log.Info("sos alert emails sent", "messages", len(batch), "tests", len(batch)-len(real), "recipients", sent)
}

// sosAlertEmail renders one alert email for a batch (oldest first).
func (s *Server) sosAlertEmail(batch []store.SOSAlertMsg) (subject, text, body string) {
	base := s.mail.BaseURL()
	link := base + "/channels?channel=sos"
	settings := base + "/account#sos-alerts"
	loc, err := time.LoadLocation("America/Vancouver")
	if err != nil {
		loc = time.UTC
	}
	who := func(m store.SOSAlertMsg) string {
		name := strings.TrimSpace(m.Sender)
		if name == "" {
			name = "Unknown sender"
		}
		if r := []rune(name); len(r) > 40 {
			name = string(r[:40]) + "…"
		}
		return name
	}

	if len(batch) == 1 {
		subject = "SOS on the mesh: " + who(batch[0])
	} else {
		subject = fmt.Sprintf("SOS on the mesh: %d messages on #sos", len(batch))
	}

	shown := batch
	if len(shown) > sosAlertMaxShown {
		shown = shown[len(shown)-sosAlertMaxShown:]
	}
	var tl, hl strings.Builder
	for _, m := range shown {
		at := m.ReceivedAt.In(loc).Format("Mon Jan 2, 3:04 PM MST")
		fmt.Fprintf(&tl, "%s — %s:\n  %s\n\n", at, who(m), m.Message)
		fmt.Fprintf(&hl, `<p style="margin:0 0 4px;color:#666;font-size:13px">%s</p>
<blockquote style="margin:0 0 14px;padding:8px 14px;border-left:3px solid #d94a47;color:#222"><strong>%s</strong>: %s</blockquote>
`, html.EscapeString(at), html.EscapeString(who(m)), html.EscapeString(m.Message))
	}
	more := ""
	if extra := len(batch) - len(shown); extra > 0 {
		more = fmt.Sprintf("(and %d earlier message(s) — see the channel)", extra)
	}

	text = fmt.Sprintf(`Ridgeline heard this on the #sos channel:

%s%s
Read the channel: %s

Ridgeline only relays what the mesh hears: it can't reply or reach emergency services. To respond, use #sos in your own MeshCore app or radio. If someone's life is in danger, call 911.

You're receiving this because you turned on #sos alert emails. Turn them off under Account on Ridgeline: %s
`, tl.String(), more, link, settings)

	moreHTML := ""
	if more != "" {
		moreHTML = `<p style="color:#666;font-size:13px">` + html.EscapeString(more) + `</p>`
	}
	body = fmt.Sprintf(`<p>Ridgeline heard this on the <strong>#sos</strong> channel:</p>
%s%s
<p><a href="%s" style="display:inline-block;padding:10px 18px;background:#d94a47;color:#fff;border-radius:6px;text-decoration:none;font-weight:600">Read #sos on Ridgeline</a></p>
<p style="color:#444;font-size:13px">Ridgeline only relays what the mesh hears: it can't reply or reach emergency services. To respond, use #sos in your own MeshCore app or radio. <strong>If someone's life is in danger, call 911.</strong></p>
<p style="color:#666;font-size:13px">You're receiving this because you turned on #sos alert emails. <a href="%s">Turn them off</a> under Account on Ridgeline.</p>`,
		hl.String(), moreHTML, htmlAttr(link), htmlAttr(settings))
	return subject, text, body
}

// accountSOSAlerts turns the caller's #sos alert emails on or off, and
// whether they include test messages. Either field may be sent alone.
func (s *Server) accountSOSAlerts(w http.ResponseWriter, r *http.Request, user store.User) {
	var req struct {
		Enabled *bool `json:"enabled"`
		Tests   *bool `json:"tests"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Enabled == nil && req.Tests == nil) {
		writeErr(w, http.StatusBadRequest, "bad request body")
		return
	}
	if req.Enabled != nil {
		if err := s.store.SetSOSAlerts(user.ID, *req.Enabled); err != nil {
			s.fail(w, err)
			return
		}
	}
	if req.Tests != nil {
		if err := s.store.SetSOSAlertTests(user.ID, *req.Tests); err != nil {
			s.fail(w, err)
			return
		}
	}
	s.returnUpdatedUser(w, user.ID)
}
