package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jjkroell/ridgeline/internal/meshcore"
	"github.com/jjkroell/ridgeline/internal/store"
)

// An owner or note author with no display name is labelled by user id on the
// public claim and notes endpoints, never by account email.
func TestPublicLabelsNeverShowEmail(t *testing.T) {
	st, base, cleanup := newAuthEnv(t)
	defer cleanup()
	pkt, _ := meshcore.DecodeHex(claimAdvertHex)
	node := pkt.Advert.PublicKey
	if err := st.Record(store.Observation{Packet: pkt, RawHex: claimAdvertHex, ReceivedAt: time.Now()}); err != nil {
		t.Fatalf("record node: %v", err)
	}

	owner := newClient(t, base) // bootstrap admin, no display name
	owner.do("POST", "/api/auth/register",
		map[string]string{"email": "owner-secret@example.com", "password": "hunter2hunter2"}, false)
	_, cb := owner.do("POST", "/api/claims", map[string]string{"pubkey": node}, true)
	code, _ := cb["code"].(string)
	if v, err := st.VerifyPendingClaims(node, "Repeater "+code); err != nil || len(v) != 1 {
		t.Fatalf("verify claim: %v %v", v, err)
	}
	if resp, _ := owner.do("POST", "/api/nodes/"+node+"/notes",
		map[string]string{"body": "hello", "visibility": "public"}, true); resp.StatusCode != 200 {
		t.Fatalf("create note: %d", resp.StatusCode)
	}

	for _, path := range []string{"/claim", "/notes"} {
		resp, err := http.Get(base + "/api/nodes/" + node + path)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if strings.Contains(string(raw), "@") {
			t.Errorf("anonymous GET %s leaks an email: %s", path, raw)
		}
		if !strings.Contains(string(raw), "User #") {
			t.Errorf("anonymous GET %s: want a 'User #' label, got %s", path, raw)
		}
	}
}
