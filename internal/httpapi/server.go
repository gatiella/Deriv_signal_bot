// Package httpapi wires up the dashboard's HTTP + WebSocket surface: the
// Deriv OAuth login flow, JSON endpoints the frontend polls/calls, and the
// live websocket feed of signals/snapshots.
package httpapi

import (
	"net/http"

	"github.com/gatiella/deriv-signal-bot/internal/config"
	"github.com/gatiella/deriv-signal-bot/internal/cryptoutil"
	"github.com/gatiella/deriv-signal-bot/internal/store"
)

// Server holds everything the HTTP handlers need.
type Server struct {
	cfg    *config.Config
	store  *store.Store
	crypto *cryptoutil.Box
	hub    *Hub
	mux    *http.ServeMux
}

// New builds a Server and registers all routes.
func New(cfg *config.Config, st *store.Store, box *cryptoutil.Box, hub *Hub) *Server {
	s := &Server{cfg: cfg, store: st, crypto: box, hub: hub, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler returns the http.Handler to pass to http.ListenAndServe.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	// Static dashboard files.
	fs := http.FileServer(http.Dir("web"))
	s.mux.Handle("/static/", fs)
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "web/index.html")
	})

	// Auth (Deriv OAuth).
	s.mux.HandleFunc("GET /auth/deriv/login", s.handleLogin)
	s.mux.HandleFunc("GET /auth/deriv/callback", s.handleCallback)
	s.mux.HandleFunc("POST /auth/logout", s.handleLogout)

	// JSON API.
	s.mux.HandleFunc("GET /api/me", s.handleMe)
	s.mux.HandleFunc("GET /api/signals/recent", s.handleRecentSignals)
	s.mux.HandleFunc("GET /api/backtest", s.handleBacktest)

	// Live feed.
	s.mux.HandleFunc("GET /ws/live", s.hub.ServeWS)
}
