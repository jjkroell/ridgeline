package api

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/jjkroell/ridgeline/internal/store"
)

// The #mayday survey (v0.24.88): what people thought of the emergency-channel
// integration (#sos until then) and whether Ridgeline should run it again.
// Anyone may answer, signed in or not; answers are stored, never emailed, and
// read by admins under Admin → Survey, grouped by a salted hash of the
// sender's IP so repeat answers from one connection stand out. Only the server's own normalised copy is
// kept: every choice is checked against these lists, free text is capped.
const surveyMayday = "mayday-2026"

var (
	surveyUsed     = []string{"posted", "radio", "listed", "emails", "alarm", "read", "never"}
	surveyMonitor  = []string{"yes", "no", "unsure"}
	surveyFeatures = []string{"emails", "list", "rings", "bot", "alarm", "guide"}
)

const (
	surveyMaxText  = 2000
	surveyMaxShort = 60
)

type surveyAnswers struct {
	Used      []string `json:"used"`
	Monitor   string   `json:"monitor"`
	NameOther string   `json:"nameOther,omitempty"` // question 3: a written-in channel name
	Features  []string `json:"features"`
	Concerns  string   `json:"concerns,omitempty"`
	Other     string   `json:"other,omitempty"`
	Email     string   `json:"email,omitempty"`
}

// pick keeps the known values of vs, each once, in the list's order.
func pick(vs, allowed []string) []string {
	out := []string{}
	for _, a := range allowed {
		if slices.Contains(vs, a) {
			out = append(out, a)
		}
	}
	return out
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		s = string(r[:n])
	}
	return s
}

func (s *Server) surveySubmit(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != surveyMayday {
		writeErr(w, http.StatusNotFound, "unknown survey")
		return
	}
	var req struct {
		surveyAnswers
		Website string `json:"website"` // honeypot, as on the contact form
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request")
		return
	}
	if strings.TrimSpace(req.Website) != "" {
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	a := surveyAnswers{
		Used:     pick(req.Used, surveyUsed),
		Features: pick(req.Features, surveyFeatures),
		Concerns: clip(req.Concerns, surveyMaxText),
		Other:    clip(req.Other, surveyMaxText),
	}
	if slices.Contains(surveyMonitor, req.Monitor) {
		a.Monitor = req.Monitor
	}
	a.NameOther = clip(req.NameOther, surveyMaxShort)
	if e := clip(req.Email, 200); e != "" {
		addr, err := mail.ParseAddress(e)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "that email address doesn't look right — leave it empty if you'd rather not give one")
			return
		}
		a.Email = addr.Address
	}
	if len(a.Used) == 0 && a.Monitor == "" && a.NameOther == "" && len(a.Features) == 0 && a.Concerns == "" && a.Other == "" {
		writeErr(w, http.StatusBadRequest, "please answer at least one question")
		return
	}
	if !s.surveyLimiter.Allow(clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, "thanks — we already have answers from here; please try again later")
		return
	}
	var uid int64
	if u, _, ok := s.currentUser(r); ok {
		uid = u.ID
	}
	b, _ := json.Marshal(a)
	// A salted hash of the address, never the address: lets an admin see
	// several answers from one connection (see store.SurveyIPHash).
	ipHash, err := s.store.SurveyIPHash(clientIP(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.store.AddSurveyResponse(surveyMayday, b, uid, ipHash, time.Now()); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("survey answered", "survey", surveyMayday, "user", uid)
	writeJSON(w, map[string]bool{"ok": true})
}

// adminSurvey lists a survey's answers for admins, newest first.
func (s *Server) adminSurvey(w http.ResponseWriter, r *http.Request, _ store.User) {
	if r.PathValue("id") != surveyMayday {
		writeErr(w, http.StatusNotFound, "unknown survey")
		return
	}
	rs, err := s.store.SurveyResponses(surveyMayday)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"survey": surveyMayday, "responses": rs})
}
