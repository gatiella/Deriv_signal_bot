package deriv

// Tick mirrors the `tick` object Deriv sends on a ticks subscription.
// See: https://developers.deriv.com/docs/data/ticks-history (and the `ticks` stream).
type Tick struct {
	Symbol  string  `json:"symbol"`
	Quote   float64 `json:"quote"`
	Epoch   int64   `json:"epoch"`
	PipSize int     `json:"pip_size"`
	ID      string  `json:"id"`
}

// tickEnvelope is the top-level shape of a `tick` push message.
type tickEnvelope struct {
	MsgType string    `json:"msg_type"`
	Tick    *Tick     `json:"tick"`
	ReqID   int64     `json:"req_id"`
	Error   *APIError `json:"error"`
}

// APIError is Deriv's standard error shape, present on any failed request.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

// AuthorizeResult is the subset of the `authorize` response we care about.
type AuthorizeResult struct {
	LoginID   string  `json:"loginid"`
	Email     string  `json:"email"`
	Currency  string  `json:"currency"`
	IsVirtual int     `json:"is_virtual"`
	Balance   float64 `json:"balance"`
	Fullname  string  `json:"fullname"`
}

// HistoryCandle mirrors one entry of a ticks_history "candles" response.
type HistoryCandle struct {
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
	Epoch int64   `json:"epoch"`
}

// HistoryTicks mirrors a ticks_history "ticks" style response.
type HistoryTicks struct {
	Prices []float64 `json:"prices"`
	Times  []int64   `json:"times"`
}

// ActiveSymbolInfo is the subset of an active_symbols entry we need - mainly
// `pip`, which tells us how many decimal places a symbol quotes to (needed
// to correctly extract "the last digit" for digit contracts).
type ActiveSymbolInfo struct {
	Symbol      string  `json:"symbol"`
	DisplayName string  `json:"display_name"`
	Pip         float64 `json:"pip"`
	Market      string  `json:"market"`
}
