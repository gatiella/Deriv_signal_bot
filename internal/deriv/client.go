// Package deriv is a minimal client for Deriv's classic WebSocket API
// (wss://ws.derivws.com/websockets/v3), the same API DTrader itself runs on.
// Docs: https://developers.deriv.com/docs (and the legacy `api.deriv.com`
// reference for the full method list: ticks, ticks_history, authorize, buy,
// proposal, active_symbols, balance, etc).
//
// This client is intentionally small: it does JSON in, JSON out, and routes
// responses back to callers by req_id. It does not know anything about
// trading strategy - that lives in internal/signals.
package deriv

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Client wraps a single websocket connection to Deriv.
//
// One Client can either be:
//   - unauthenticated, for public market data (ticks, ticks_history, active_symbols)
//   - authenticated, after calling Authorize with a user's API token
//
// A single connection can only be authorized as one account at a time, so
// the server keeps one long-lived public Client for market data (shared by
// all users) and opens short-lived authenticated Clients on demand for
// account-specific calls like balance.
type Client struct {
	url  string
	conn *websocket.Conn

	mu      sync.Mutex
	nextReq int64
	pending map[int64]chan json.RawMessage // one-shot requests: closed & deleted after first reply
	streams map[int64]chan json.RawMessage // subscriptions: kept open until Unsubscribe

	closeOnce sync.Once
	closed    chan struct{}
}

// Connect dials the given Deriv websocket URL (include ?app_id=... in the URL).
func Connect(url string) (*Client, error) {
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()
			return nil, fmt.Errorf("dial deriv websocket: %w (http %d, body: %q)", err, resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("dial deriv websocket: %w (no http response - connection likely blocked before reaching Deriv)", err)
	}
	c := &Client{
		url:     url,
		conn:    conn,
		pending: make(map[int64]chan json.RawMessage),
		streams: make(map[int64]chan json.RawMessage),
		closed:  make(chan struct{}),
	}
	go c.readLoop()
	go c.pingLoop()
	return c, nil
}

// Close shuts down the underlying connection.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closed)
		err = c.conn.Close()
	})
	return err
}

// pingLoop keeps the connection alive; Deriv closes idle sockets after a
// while, so we ping every 30s as recommended by their docs.
func (c *Client) pingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.closed:
			return
		case <-ticker.C:
			if _, err := c.Call(map[string]interface{}{"ping": 1}); err != nil {
				log.Printf("deriv: ping failed: %v", err)
				return
			}
		}
	}
}

// readLoop continuously reads frames and routes them to the right waiter by
// req_id. Deriv includes the original req_id in every reply, including every
// push belonging to a subscription started with that req_id.
func (c *Client) readLoop() {
	defer c.Close()
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("deriv: read loop ending: %v", err)
			c.mu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			for _, ch := range c.streams {
				close(ch)
			}
			c.pending = map[int64]chan json.RawMessage{}
			c.streams = map[int64]chan json.RawMessage{}
			c.mu.Unlock()
			return
		}

		var envelope struct {
			ReqID int64 `json:"req_id"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			log.Printf("deriv: could not parse envelope: %v", err)
			continue
		}

		c.mu.Lock()
		if ch, ok := c.pending[envelope.ReqID]; ok {
			ch <- data
			close(ch)
			delete(c.pending, envelope.ReqID)
		} else if ch, ok := c.streams[envelope.ReqID]; ok {
			// Non-blocking send: a slow consumer should not stall the read loop.
			select {
			case ch <- data:
			default:
				log.Printf("deriv: dropped message for slow subscriber (req_id=%d)", envelope.ReqID)
			}
		}
		c.mu.Unlock()
	}
}

// Call sends a one-off request and waits for its single reply.
func (c *Client) Call(req map[string]interface{}) (json.RawMessage, error) {
	reqID := atomic.AddInt64(&c.nextReq, 1)
	req["req_id"] = reqID

	ch := make(chan json.RawMessage, 1)
	c.mu.Lock()
	c.pending[reqID] = ch
	c.mu.Unlock()

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	writeErr := c.conn.WriteMessage(websocket.TextMessage, body)
	c.mu.Unlock()
	if writeErr != nil {
		return nil, writeErr
	}

	select {
	case data, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("connection closed while waiting for response")
		}
		if apiErr := extractError(data); apiErr != nil {
			return nil, apiErr
		}
		return data, nil
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("timed out waiting for deriv response")
	}
}

// Subscribe sends a request with subscribe:1 and returns a channel that
// receives every push for that subscription, plus an Unsubscribe function.
func (c *Client) Subscribe(req map[string]interface{}) (<-chan json.RawMessage, func(), error) {
	req["subscribe"] = 1
	reqID := atomic.AddInt64(&c.nextReq, 1)
	req["req_id"] = reqID

	ch := make(chan json.RawMessage, 32)
	c.mu.Lock()
	c.streams[reqID] = ch
	c.mu.Unlock()

	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	c.mu.Lock()
	writeErr := c.conn.WriteMessage(websocket.TextMessage, body)
	c.mu.Unlock()
	if writeErr != nil {
		return nil, nil, writeErr
	}

	unsubscribe := func() {
		c.mu.Lock()
		delete(c.streams, reqID)
		c.mu.Unlock()
		_, _ = c.Call(map[string]interface{}{"forget_all": "ticks"})
	}
	return ch, unsubscribe, nil
}

func extractError(data json.RawMessage) *APIError {
	var e struct {
		Error *APIError `json:"error"`
	}
	if err := json.Unmarshal(data, &e); err != nil {
		return nil
	}
	return e.Error
}

// Authorize authenticates this connection as the account owning apiToken.
// Only call this on a connection you intend to use for exactly one account.
func (c *Client) Authorize(apiToken string) (*AuthorizeResult, error) {
	data, err := c.Call(map[string]interface{}{"authorize": apiToken})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Authorize AuthorizeResult `json:"authorize"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.Authorize, nil
}

// Balance fetches the current balance for the authorized account.
func (c *Client) Balance() (float64, string, error) {
	data, err := c.Call(map[string]interface{}{"balance": 1})
	if err != nil {
		return 0, "", err
	}
	var resp struct {
		Balance struct {
			Balance  float64 `json:"balance"`
			Currency string  `json:"currency"`
		} `json:"balance"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, "", err
	}
	return resp.Balance.Balance, resp.Balance.Currency, nil
}

// SubscribeTicks streams live ticks for a symbol, e.g. "R_100".
func (c *Client) SubscribeTicks(symbol string) (<-chan *Tick, func(), error) {
	raw, unsub, err := c.Subscribe(map[string]interface{}{"ticks": symbol})
	if err != nil {
		return nil, nil, err
	}
	out := make(chan *Tick, 32)
	go func() {
		defer close(out)
		for data := range raw {
			var env tickEnvelope
			if err := json.Unmarshal(data, &env); err != nil {
				continue
			}
			if env.Tick != nil {
				out <- env.Tick
			}
		}
	}()
	return out, unsub, nil
}

// TicksHistory fetches historical candles for backtesting. granularitySecs
// is the candle size (e.g. 60 for 1-minute candles); count is how many
// candles to return (Deriv caps this, typically at 5000).
func (c *Client) TicksHistoryCandles(symbol string, granularitySecs, count int) ([]HistoryCandle, error) {
	data, err := c.Call(map[string]interface{}{
		"ticks_history": symbol,
		"end":           "latest",
		"count":         count,
		"style":         "candles",
		"granularity":   granularitySecs,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Candles []HistoryCandle `json:"candles"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Candles, nil
}

// ActiveSymbols fetches the list of tradable symbols, including each one's
// `pip` value (used to figure out decimal places for digit extraction).
func (c *Client) ActiveSymbols() ([]ActiveSymbolInfo, error) {
	data, err := c.Call(map[string]interface{}{
		"active_symbols": "brief",
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		ActiveSymbols []ActiveSymbolInfo `json:"active_symbols"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.ActiveSymbols, nil
}

// TicksHistoryTicks fetches raw historical ticks (not candles) - this is
// what the backtester uses, since digit analysis needs individual quotes,
// not OHLC bars.
func (c *Client) TicksHistoryTicks(symbol string, count int) (*HistoryTicks, error) {
	data, err := c.Call(map[string]interface{}{
		"ticks_history": symbol,
		"end":           "latest",
		"count":         count,
		"style":         "ticks",
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		History HistoryTicks `json:"history"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.History, nil
}