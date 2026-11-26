package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// The #mayday survey: anyone may answer; only known choices are kept, the
// honeypot is swallowed, an empty form is refused, and only admins read it.
func TestSurvey(t *testing.T) {
	st, base, _, cleanup := mailEnv(t)
	defer cleanup()
	owner, anon := newClient(t, base), newClient(t, base)
	owner.do("POST", "/api/auth/register",
		map[string]string{"email": "owner@example.com", "password": "hunter2hunter2"}, false)
	type m = map[string]any
	post := func(body m, want int) {
		t.Helper()
		if r, b := anon.do("POST", "/api/survey/mayday-2026", body, false); r.StatusCode != want {
			t.Fatalf("%v: %d %v, want %d", body, r.StatusCode, b, want)
		}
	}
	post(m{}, 400)
	post(m{"monitor": "yes", "email": "not an address"}, 400)
	post(m{"monitor": "yes", "website": "spam.example"}, 200) // honeypot: ok, not stored
	post(m{"used": []string{"read", "bogus", "posted"}, "monitor": "maybe",
		"nameOther": "#help", "features": []string{"emails"}, "concerns": "  false alarms ", "email": "A <a@x.ca>"}, 200)
	if r, _ := anon.do("POST", "/api/survey/other", m{"monitor": "yes"}, false); r.StatusCode != 404 {
		t.Fatalf("unknown survey: %d", r.StatusCode)
	}
	rs, _ := st.SurveyResponses(surveyMayday)
	if len(rs) != 1 {
		t.Fatalf("stored %d", len(rs))
	}
	var a surveyAnswers
	json.Unmarshal(rs[0].Answers, &a)
	if len(a.Used) != 2 || a.Used[0] != "posted" || a.Monitor != "" || a.NameOther != "#help" ||
		a.Concerns != "false alarms" || a.Email != "a@x.ca" {
		t.Fatalf("answers = %+v", a)
	}
	if r, _ := anon.do("GET", "/api/admin/survey/mayday-2026", nil, false); r.StatusCode != 401 && r.StatusCode != 403 {
		t.Fatalf("anon read: %d", r.StatusCode)
	}
	if r, b := owner.do("GET", "/api/admin/survey/mayday-2026", nil, false); r.StatusCode != 200 || len(b["responses"].([]any)) != 1 {
		t.Fatalf("admin read: %d %v", r.StatusCode, b)
	}
	// A second answer from the same connection shares the group; the IP isn't stored.
	post(m{"monitor": "no"}, 200)
	rs, _ = st.SurveyResponses(surveyMayday)
	if len(rs) != 2 || rs[0].Group == "" || rs[0].Group != rs[1].Group {
		t.Fatalf("groups = %q %q", rs[0].Group, rs[1].Group)
	}
	h1, _ := st.SurveyIPHash("203.0.113.9")
	h2, _ := st.SurveyIPHash("203.0.113.9")
	if h1 != h2 || h1 == "" || strings.Contains(h1, "203") {
		t.Fatalf("hash = %q / %q", h1, h2)
	}
}
