package httpapi

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Dashboard is served same-origin; if you deploy the frontend on a
	// different domain, tighten this to check r.Header.Get("Origin").
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Hub fans out JSON messages (snapshots and signals) to every connected
// dashboard client. Market data is public, so there's no per-user filtering
// here - every connected browser sees the same live feed.
type Hub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[chan []byte]struct{})}
}

// Broadcast sends msg to every currently-connected client. Slow clients get
// dropped rather than blocking everyone else.
func (h *Hub) Broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// client too slow, drop this message for them
		}
	}
}

// ServeWS upgrades the request to a websocket and streams broadcasts to it
// until the client disconnects.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("wshub: upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
	}()

	// Drain incoming messages (we don't expect any, but this detects
	// disconnects promptly and keeps the read side of the socket healthy).
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				conn.Close()
				return
			}
		}
	}()

	for msg := range ch {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}
