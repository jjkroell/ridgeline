package api

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"strings"
)

// Contact form: a way for operators to reach the network's maintainer from the
// public pages, without publishing an address for scrapers to harvest.
//
// It sends to one fixed, configured recipient and nowhere else. The submitter
// chooses the message, never the destination — an open relay is the failure
// mode this endpoint has to avoid, and the only defence that actually works is
// not letting the request name a recipient at all.
//
// Off unless contactTo is configured, so an instance that has not opted in
// answers 503 rather than silently swallowing messages.

// contactMaxMessage bounds the body. Long enough for a detailed report of a
// node misbehaving, short enough that the endpoint cannot be used to push bulk
// content through the relay.
const contactMaxMessage = 4000

// SetContactTo names the mailbox the contact form delivers to. Empty (the
// default) disables the endpoint.
func (s *Server) SetContactTo(addr string) { s.contactTo = strings.TrimSpace(addr) }

type contactReq struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
	// Website is a honeypot. It is hidden from people by CSS and left empty by
	// every real submission; a bot that fills every field it finds trips it.
	// Deliberately named something a scraper would want to autofill.
	Website string `json:"website"`
}

func (s *Server) contact(w http.ResponseWriter, r *http.Request) {
	if s.contactTo == "" || !s.mailEnabled() {
		writeJSONStatus(w, http.StatusServiceUnavailable,
			map[string]string{"error": "the contact form is not configured on this instance"})
		return
	}

	var req contactReq
	// 16 KB: comfortably above the field caps below, and a bound on what an
	// unauthenticated caller can make the decoder allocate.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "malformed request"})
		return
	}

	// Honeypot: answer exactly as a success would, and send nothing. Telling a
	// bot it was caught only teaches whoever wrote it to stop filling the field.
	if strings.TrimSpace(req.Website) != "" {
		writeJSON(w, map[string]bool{"ok": true})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	req.Message = strings.TrimSpace(req.Message)

	if req.Message == "" || len([]rune(req.Message)) > contactMaxMessage {
		writeJSONStatus(w, http.StatusBadRequest,
			map[string]string{"error": fmt.Sprintf("message is required, and at most %d characters", contactMaxMessage)})
		return
	}
	// A reply address is required: a report nobody can answer is worth little,
	// and the alternative is a mailbox of anonymous notes with no way to ask a
	// follow-up question.
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeJSONStatus(w, http.StatusBadRequest,
			map[string]string{"error": "a valid email address is required so we can reply"})
		return
	}
	if len([]rune(req.Name)) > 100 {
		req.Name = string([]rune(req.Name)[:100])
	}

	// Same limiters as registration and password reset: per-IP, then per
	// submitting address. Checked before any work runs.
	if !s.emailRateOK(r, req.Email) {
		writeJSONStatus(w, http.StatusTooManyRequests,
			map[string]string{"error": "too many messages from here — please try again later"})
		return
	}

	from := req.Email
	if req.Name != "" {
		from = req.Name + " <" + req.Email + ">"
	}
	subject := fmt.Sprintf("[Ridgeline] Message from %s", from)

	// The submitter's address leads the body rather than riding in a Reply-To
	// header: the mailer sets Reply-To from its own config for every message it
	// sends, and bending that for one endpoint would change the headers of the
	// verification and notification mail too. The HTML part carries a mailto:
	// so replying is still one click.
	text := fmt.Sprintf("From: %s\nReply to: %s\n\n%s\n", from, req.Email, req.Message)
	htmlBody := fmt.Sprintf(
		`<p>From: %s<br>Reply to: <a href="mailto:%s">%s</a></p><hr><p style="white-space:pre-wrap">%s</p>`,
		html.EscapeString(from),
		html.EscapeString(req.Email), html.EscapeString(req.Email),
		html.EscapeString(req.Message))

	s.mail.SendAsync("contact", s.contactTo, subject, text, htmlBody)
	s.log.Info("contact form submitted", "from", req.Email)
	writeJSON(w, map[string]bool{"ok": true})
}
