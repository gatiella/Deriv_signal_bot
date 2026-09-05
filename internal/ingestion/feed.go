// Package ingestion owns the single shared connection to Deriv's public
// market data feed. Market data doesn't require authentication, so one
// connection serves ticks for every user of the dashboard - there's no need
// for a per-user connection here.
package ingestion

import (
	"fmt"
	"log"
	"time"

	"github.com/gatiella/deriv-signal-bot/internal/deriv"
	"github.com/gatiella/deriv-signal-bot/internal/signals"
)

// Symbols is the fixed set of classic Volatility Indices this tool watches.
var Symbols = []string{"R_10", "R_25", "R_50", "R_75", "R_100"}

// Feed connects to Deriv, subscribes to ticks for every symbol in Symbols,
// and forwards each tick into the signal engine. It automatically
// reconnects with backoff if the connection drops.
type Feed struct {
	wsURL  string
	appID  string
	engine *signals.Engine
}

// New builds a Feed. wsURL should be the base Deriv websocket URL (without
// app_id query param - New adds it).
func New(wsURL, appID string, engine *signals.Engine) *Feed {
	return &Feed{wsURL: wsURL, appID: appID, engine: engine}
}

// Run blocks, keeping the feed alive until the process exits. Call it in a
// goroutine from main.
func (f *Feed) Run() {
	backoff := time.Second
	for {
		if err := f.runOnce(); err != nil {
			log.Printf("ingestion: feed error: %v (retrying in %s)", err, backoff)
		}
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (f *Feed) runOnce() error {
	url := fmt.Sprintf("%s?app_id=%s", f.wsURL, f.appID)
	client, err := deriv.Connect(url)
	if err != nil {
		return err
	}
	defer client.Close()
	log.Printf("ingestion: connected to Deriv, subscribing to %v", Symbols)

	done := make(chan error, len(Symbols))
	for _, symbol := range Symbols {
		symbol := symbol
		ticks, _, err := client.SubscribeTicks(symbol)
		if err != nil {
			return fmt.Errorf("subscribe %s: %w", symbol, err)
		}
		go func() {
			for tick := range ticks {
				f.engine.Feed(tick.Symbol, tick.Quote, tick.PipSize)
			}
			done <- fmt.Errorf("tick stream for %s ended", symbol)
		}()
	}

	// Block until any one stream ends (which happens if the connection drops),
	// then let runOnce return so Run() reconnects everything cleanly.
	return <-done
}
