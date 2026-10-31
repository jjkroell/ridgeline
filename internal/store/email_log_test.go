package store

import (
	"errors"
	"testing"
)

func TestSOSAlertSettingChangesAreRecorded(t *testing.T) {
	st := testStore(t)
	u, err := st.CreateUser("a@example.com", "h", "A")
	if err != nil {
		t.Fatal(err)
	}
	count := func() (n int) {
		st.db.QueryRow(`SELECT COUNT(*) FROM sos_alert_changes WHERE user_id = ?`, u.ID).Scan(&n)
		return n
	}

	if err := st.SetSOSAlerts(u.ID, true); err != nil {
		t.Fatal(err)
	}
	got, _, _ := st.GetUserByID(u.ID)
	if !got.SOSAlerts || got.SOSAlertsAt == "" || got.SOSAlertTestsAt != "" {
		t.Fatalf("after turning alerts on: %+v", got)
	}
	first := got.SOSAlertsAt

	// Saving the same value again is not a change.
	if err := st.SetSOSAlerts(u.ID, true); err != nil {
		t.Fatal(err)
	}
	if got, _, _ = st.GetUserByID(u.ID); got.SOSAlertsAt != first || count() != 1 {
		t.Fatalf("re-save moved the stamp or logged: at=%q rows=%d", got.SOSAlertsAt, count())
	}

	if err := st.SetSOSAlertTests(u.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSOSAlerts(u.ID, false); err != nil {
		t.Fatal(err)
	}
	if got, _, _ = st.GetUserByID(u.ID); got.SOSAlerts || !got.SOSAlertTests || got.SOSAlertTestsAt == "" {
		t.Fatalf("after tests on, alerts off: %+v", got)
	}
	if n := count(); n != 3 {
		t.Fatalf("want 3 change rows, got %d", n)
	}
	var setting string
	var value int
	st.db.QueryRow(`SELECT setting, value FROM sos_alert_changes WHERE user_id = ? ORDER BY id DESC LIMIT 1`, u.ID).
		Scan(&setting, &value)
	if setting != "alerts" || value != 0 {
		t.Fatalf("last change = %s/%d, want alerts/0", setting, value)
	}
}

func TestLogEmail(t *testing.T) {
	st := testStore(t)
	if err := st.LogEmail("sos-alert", "x@example.com", "SOS on the mesh: X", nil); err != nil {
		t.Fatal(err)
	}
	if err := st.LogEmail("verification", "y@example.com", "Confirm", errors.New("smtp rcpt: 550 no")); err != nil {
		t.Fatal(err)
	}
	var n int
	st.db.QueryRow(`SELECT COUNT(*) FROM email_log`).Scan(&n)
	var failed string
	st.db.QueryRow(`SELECT error FROM email_log WHERE recipient = 'y@example.com'`).Scan(&failed)
	if n != 2 || failed != "smtp rcpt: 550 no" {
		t.Fatalf("rows=%d error=%q", n, failed)
	}
	st.db.Exec(`UPDATE email_log SET sent_at = '2020-01-01T00:00:00Z' WHERE recipient = 'x@example.com'`)
	if pruned, err := st.PruneEmailLog(); err != nil || pruned != 1 {
		t.Fatalf("prune: %d %v", pruned, err)
	}
}

func TestListEmailLog(t *testing.T) {
	st := testStore(t)
	for i, to := range []string{"a@x.com", "b@y.com", "a@x.com", "c@x.com"} {
		kind := "sos-alert"
		if i == 1 {
			kind = "verification"
		}
		if err := st.LogEmail(kind, to, "s", nil); err != nil {
			t.Fatal(err)
		}
	}
	all, more, err := st.ListEmailLog("", "", 0, 3)
	if err != nil || len(all) != 3 || !more || all[0].Recipient != "c@x.com" {
		t.Fatalf("first page: %+v more=%v err=%v", all, more, err)
	}
	rest, more, _ := st.ListEmailLog("", "", all[2].ID, 3)
	if len(rest) != 1 || more || rest[0].Recipient != "a@x.com" {
		t.Fatalf("second page: %+v more=%v", rest, more)
	}
	if got, _, _ := st.ListEmailLog("verification", "", 0, 10); len(got) != 1 || got[0].Recipient != "b@y.com" {
		t.Fatalf("kind filter: %+v", got)
	}
	if got, _, _ := st.ListEmailLog("", "A@X", 0, 10); len(got) != 2 {
		t.Fatalf("recipient search: %+v", got)
	}
}
