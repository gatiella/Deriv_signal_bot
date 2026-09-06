// Command server runs the Deriv Volatility signal dashboard: it connects to
// Deriv's public market feed for R_10/25/50/75/100, tracks digit frequency
// and tick momentum, serves a live dashboard over HTTP/WebSocket, and lets
// users connect their own Deriv account via OAuth to see their balance.
//
// See README.md for setup instructions (registering a Deriv app, database,
// environment variables) before running this.
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gatiella/deriv-signal-bot/internal/config"
	"github.com/gatiella/deriv-signal-bot/internal/cryptoutil"
	"github.com/gatiella/deriv-signal-bot/internal/httpapi"
	"github.com/gatiella/deriv-signal-bot/internal/ingestion"
	"github.com/gatiella/deriv-signal-bot/internal/signals"
	"github.com/gatiella/deriv-signal-bot/internal/store"
)

// wireMessage is the envelope every message pushed over /ws/live uses, so
// the frontend can dispatch on "type" without guessing.
type wireMessage struct {
	Type string      `json:"type"` // "snapshot" | "signal"
	Data interface{} `json:"data"`
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	box, err := cryptoutil.NewBox(cfg.AppSecretKeyB64)
	if err != nil {
		log.Fatalf("crypto: %v", err)
	}

	st, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	hub := httpapi.NewHub()

	engine := signals.NewEngine(
		signals.EngineConfig{
			DigitWindowSize:    cfg.DigitWindowSize,
			DigitThresholdPct:  cfg.DigitThresholdPct,
			MomentumWindowSize: cfg.MomentumWindowSize,
			MomentumThreshold:  cfg.MomentumThreshold,
		},
		func(sig signals.Signal) {
			if err := st.SaveSignal(sig); err != nil {
				log.Printf("main: save signal: %v", err)
			}
			broadcast(hub, "signal", sig)
		},
		func(snap signals.Snapshot) {
			broadcast(hub, "snapshot", snap)
		},
	)

	feed := ingestion.New(cfg.DerivWSURL, cfg.AppID, engine)
	go feed.Run()

	srv := httpapi.New(cfg, st, box, hub)

	log.Printf("deriv-signal-bot listening on %s", cfg.HTTPAddr)
	log.Printf("watching symbols: %v", ingestion.Symbols)
	if err := http.ListenAndServe(cfg.HTTPAddr, srv.Handler()); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

func broadcast(hub *httpapi.Hub, msgType string, data interface{}) {
	body, err := json.Marshal(wireMessage{Type: msgType, Data: data})
	if err != nil {
		log.Printf("main: marshal broadcast: %v", err)
		return
	}
	hub.Broadcast(body)
}
