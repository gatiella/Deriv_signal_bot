package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gatiella/deriv-signal-bot/internal/backtest"
	"github.com/gatiella/deriv-signal-bot/internal/deriv"
	"github.com/gatiella/deriv-signal-bot/internal/ingestion"
)

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("httpapi: encode response: %v", err)
	}
}

// handleRecentSignals returns the most recent edge-triggered signals, newest
// first. Query params: symbol (optional filter), limit (default 50, max 200).
func (s *Server) handleRecentSignals(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.store.RecentSignals(symbol, limit)
	if err != nil {
		log.Printf("handleRecentSignals: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, rows)
}

// handleBacktest runs backtest.Run against real historical ticks and returns
// the honest win-rate report for every rule this tool tracks. Query params:
// symbol (required, one of the watched Volatility symbols), sample (optional,
// default 3000, max 5000 - Deriv's own cap on ticks_history).
func (s *Server) handleBacktest(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if !isWatchedSymbol(symbol) {
		http.Error(w, "symbol must be one of "+joinSymbols(), http.StatusBadRequest)
		return
	}
	sample := 3000
	if v := r.URL.Query().Get("sample"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sample = n
		}
	}
	if sample > 5000 {
		sample = 5000
	}

	wsURL := s.cfg.DerivWSURL + "?app_id=" + s.cfg.AppID
	client, err := deriv.Connect(wsURL)
	if err != nil {
		log.Printf("handleBacktest: connect: %v", err)
		http.Error(w, "could not reach Deriv", http.StatusBadGateway)
		return
	}
	defer client.Close()

	report, err := backtest.Run(client, backtest.Config{
		Symbol:             symbol,
		SampleSize:         sample,
		DigitWindowSize:    s.cfg.DigitWindowSize,
		DigitThresholdPct:  s.cfg.DigitThresholdPct,
		MomentumWindowSize: s.cfg.MomentumWindowSize,
		MomentumThreshold:  s.cfg.MomentumThreshold,
	})
	if err != nil {
		log.Printf("handleBacktest: run: %v", err)
		http.Error(w, "backtest failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, report)
}

func isWatchedSymbol(symbol string) bool {
	for _, s := range ingestion.Symbols {
		if s == symbol {
			return true
		}
	}
	return false
}

func joinSymbols() string {
	out := ""
	for i, s := range ingestion.Symbols {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}