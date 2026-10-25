package api

import (
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
