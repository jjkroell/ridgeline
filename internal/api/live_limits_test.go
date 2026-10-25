package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// A client sending more than liveReadLimit is disconnected instead of having
// the server buffer the message.
func TestLiveSocketClosesOnOversizedMessage(t *testing.T) {
	_, base, cleanup := newAuthEnv(t)
	defer cleanup()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(base, "http")+"/api/live", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, make([]byte, 64<<10)); err != nil {
		return // the server dropped us mid-message
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			if websocket.IsCloseError(err, websocket.CloseMessageTooBig) || !strings.Contains(err.Error(), "timeout") {
				return // closed by the server
			}
			t.Fatalf("connection still open after an oversized message: %v", err)
		}
	}
}

// One visitor IP gets at most maxLiveClientsPerIP live sockets; another IP is
// unaffected, and closing a socket frees its slot.
func TestLiveSocketPerIPCap(t *testing.T) {
	_, base, cleanup := newAuthEnv(t)
	defer cleanup()
	url := "ws" + strings.TrimPrefix(base, "http") + "/api/live"
	dial := func(ip string) (*websocket.Conn, int) {
		h := http.Header{}
		h.Set("CF-Connecting-IP", ip) // test server peer is loopback, so this is believed
		c, resp, err := websocket.DefaultDialer.Dial(url, h)
		if err != nil {
			code := 0
			if resp != nil {
				code = resp.StatusCode
			}
			return nil, code
		}
		return c, http.StatusSwitchingProtocols
	}
	var conns []*websocket.Conn
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()
	for i := 0; i < maxLiveClientsPerIP; i++ {
		c, code := dial("198.51.100.1")
		if c == nil {
			t.Fatalf("socket %d refused (%d) within the cap", i, code)
		}
		conns = append(conns, c)
	}
	if c, code := dial("198.51.100.1"); c != nil || code != http.StatusServiceUnavailable {
		t.Fatalf("socket over the per-IP cap: got %d, want 503", code)
	}
	other, _ := dial("198.51.100.2")
	if other == nil {
		t.Fatal("a different IP was refused")
	}
	other.Close()

	conns[0].Close()
	conns = conns[1:]
	deadline := time.Now().Add(5 * time.Second)
	for {
		if c, _ := dial("198.51.100.1"); c != nil {
			conns = append(conns, c)
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("closing a socket never freed its slot")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
