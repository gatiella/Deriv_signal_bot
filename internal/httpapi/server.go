// Package httpapi wires up the dashboard's HTTP + WebSocket surface: JSON
// endpoints the frontend polls/calls, and the live websocket feed of
// signals/snapshots. Account linking (Deriv OAuth login) is not included in
// this build - see README for why - so there's no auth/session layer here.
package httpapi

import (
	"net/http"

	"github.com/gatiella/deriv-signal-bot/internal/config"
	"github.com/gatiella/deriv-signal-bot/internal/store"
)

// Server holds everything the HTTP handlers need.
type Server struct {
	cfg   *config.Config
	store *store.Store
	hub   *Hub
	mux   *http.ServeMux
}

// New builds a Server and registers all routes.
func New(cfg *config.Config, st *store.Store, hub *Hub) *Server {
	s := &Server{cfg: cfg, store: st, hub: hub, mux: http.NewServeMux()}
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

	// JSON API.
	s.mux.HandleFunc("GET /api/signals/recent", s.handleRecentSignals)
	s.mux.HandleFunc("GET /api/backtest", s.handleBacktest)

	// Live feed.
	s.mux.HandleFunc("GET /ws/live", s.hub.ServeWS)
}