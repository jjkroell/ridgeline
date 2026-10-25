package api

import (
	"sync"

	"github.com/gorilla/websocket"
)

// hub fans out live observation events to all connected WebSocket clients.
type hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]chan []byte
	// slots counts admitted connections (reserved before the upgrade, released
	// when the handler returns), in total and per client IP.
	slots int
	perIP map[string]int
}

func newHub() *hub {
	return &hub{clients: make(map[*websocket.Conn]chan []byte), perIP: make(map[string]int)}
}

// reserve admits one more connection from ip if neither the global cap nor
// that IP's cap is reached. Check and claim happen under one lock, so a burst
// of concurrent upgrades cannot overshoot either cap. Pair with release.
func (h *hub) reserve(ip string, maxTotal, maxPerIP int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.slots >= maxTotal || h.perIP[ip] >= maxPerIP {
		return false
	}
	h.slots++
	h.perIP[ip]++
	return true
}

func (h *hub) release(ip string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.slots--
	if h.perIP[ip]--; h.perIP[ip] <= 0 {
		delete(h.perIP, ip)
	}
}

// add registers a connection and returns its outbound buffer channel.
func (h *hub) add(c *websocket.Conn) chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[c] = ch
	h.mu.Unlock()
	return ch
}

func (h *hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	if ch, ok := h.clients[c]; ok {
		close(ch)
		delete(h.clients, c)
	}
	h.mu.Unlock()
}

// broadcast sends a message to every connected client, dropping it for any
// client whose buffer is full rather than blocking ingest.
func (h *hub) broadcast(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.clients {
		select {
		case ch <- msg:
		default: // slow client — drop this frame
		}
	}
}
