package signals

import "time"

// ContractType mirrors the Deriv contract_type values this tool watches.
type ContractType string

const (
	DigitOver  ContractType = "DIGITOVER"
	DigitUnder ContractType = "DIGITUNDER"
	DigitEven  ContractType = "DIGITEVEN"
	DigitOdd   ContractType = "DIGITODD"
	Rise       ContractType = "CALL" // Deriv's Rise/Fall are contract_type CALL/PUT
	Fall       ContractType = "PUT"
)

// Signal is one flagged observation: a symbol's stats crossed the configured
// threshold for a given contract type. It is descriptive (see honesty notes
// in digitstats.go / momentum.go), not a guarantee.
type Signal struct {
	Symbol       string       `json:"symbol"`
	ContractType ContractType `json:"contract_type"`
	Detail       string       `json:"detail"`   // human-readable explanation, e.g. "digit 7 at 15.0% vs 10% expected"
	Strength     float64      `json:"strength"` // magnitude of the deviation that triggered this
	WindowSize   int          `json:"window_size"`
	CreatedAt    time.Time    `json:"created_at"`
}

// SnapshotDigit is a single digit's current frequency, used for the live
// dashboard bar chart.
type SnapshotDigit struct {
	Digit     int     `json:"digit"`
	Frequency float64 `json:"frequency"`
	Hot       bool    `json:"hot"`
	Cold      bool    `json:"cold"`
}

// Snapshot is the full current picture for one symbol, pushed to the
// dashboard on every update.
type Snapshot struct {
	Symbol      string          `json:"symbol"`
	Digits      []SnapshotDigit `json:"digits"`
	EvenPct     float64         `json:"even_pct"`
	OddPct      float64         `json:"odd_pct"`
	UpFrac      float64         `json:"up_frac"`
	DownFrac    float64         `json:"down_frac"`
	LastQuote   float64         `json:"last_quote"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DigitWindow int             `json:"digit_window"`
	TickWindow  int             `json:"tick_window"`
}
