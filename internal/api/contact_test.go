package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjkroell/ridgeline/internal/store"
)

// contactEnv is mailEnv plus a configured contact mailbox. Separate from
// mailEnv because the address has to be set on the Server, which mailEnv does
// not hand back.
func contactEnv(t *testing.T, to string) (string, *fakeMailer, func()) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	srv := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "")
	fm := &fakeMailer{base: "https://ridgeline.ve7kod.ca"}
	srv.SetMailer(fm)
	srv.SetContactTo(to)
	ts := httptest.NewServer(srv.Handler())
	return ts.URL, fm, func() { ts.Close(); st.Close() }
}

func postContact(t *testing.T, base, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(base+"/api/contact", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	return resp
}

func TestContactDisabledWithoutRecipient(t *testing.T) {
	base, fm, done := contactEnv(t, "")
	defer done()

	resp := postContact(t, base, `{"email":"a@b.com","message":"hi"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when no recipient is configured, got %d", resp.StatusCode)
	}
	if n := len(fm.ofKind("contact")); n != 0 {
		t.Fatalf("sent %d messages with no recipient configured", n)
	}
}

func TestContactDeliversToConfiguredMailboxOnly(t *testing.T) {
	base, fm, done := contactEnv(t, "owner@example.org")
	defer done()

	// "to" in the body is not a field the handler knows; if a future refactor
	// ever honoured it this test fails, which is the point — the endpoint must
	// never become a relay the caller can aim.
	resp := postContact(t, base,
		`{"name":"Pat","email":"pat@example.com","message":"node is deaf","to":"victim@elsewhere.net"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	sent := fm.ofKind("contact")
	if len(sent) != 1 {
		t.Fatalf("want exactly 1 message, got %d", len(sent))
	}
	if sent[0].to != "owner@example.org" {
		t.Fatalf("delivered to %q, want the configured mailbox", sent[0].to)
	}
	if !strings.Contains(sent[0].subject, "pat@example.com") {
		t.Fatalf("subject lost the sender: %q", sent[0].subject)
	}
	if !strings.Contains(sent[0].text, "node is deaf") {
		t.Fatalf("body lost the message: %q", sent[0].text)
	}
}

func TestContactHoneypotLooksLikeSuccessAndSendsNothing(t *testing.T) {
	base, fm, done := contactEnv(t, "owner@example.org")
	defer done()

	resp := postContact(t, base,
		`{"email":"bot@spam.example","message":"buy things","website":"http://spam"}`)
	defer resp.Body.Close()
	// Answering 200 is deliberate: a bot told it was caught learns to stop
	// filling the field.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 for a tripped honeypot, got %d", resp.StatusCode)
	}
	if n := len(fm.ofKind("contact")); n != 0 {
		t.Fatalf("honeypot submission sent %d messages", n)
	}
}

func TestContactRejectsUnusableInput(t *testing.T) {
	base, fm, done := contactEnv(t, "owner@example.org")
	defer done()

	for name, body := range map[string]string{
		"no reply address": `{"email":"","message":"hello"}`,
		"bad address":      `{"email":"not-an-address","message":"hello"}`,
		"empty message":    `{"email":"a@b.com","message":"   "}`,
		"malformed json":   `{"email":`,
	} {
		resp := postContact(t, base, body)
		if resp.StatusCode != http.StatusBadRequest {
			resp.Body.Close()
			t.Fatalf("%s: want 400, got %d", name, resp.StatusCode)
		}
		resp.Body.Close()
	}
	if n := len(fm.ofKind("contact")); n != 0 {
		t.Fatalf("rejected input still sent %d messages", n)
	}
}

func TestContactCapsMessageLength(t *testing.T) {
	base, fm, done := contactEnv(t, "owner@example.org")
	defer done()

	resp := postContact(t, base,
		`{"email":"a@b.com","message":"`+strings.Repeat("x", contactMaxMessage+1)+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for an over-long message, got %d", resp.StatusCode)
	}
	if n := len(fm.ofKind("contact")); n != 0 {
		t.Fatalf("over-long message was sent")
	}
}
