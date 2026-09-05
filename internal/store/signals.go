package store

import (
	"database/sql"
	"time"

	"github.com/gatiella/deriv-signal-bot/internal/signals"
)

// SaveSignal persists one emitted signal for history/analysis.
func (s *Store) SaveSignal(sig signals.Signal) error {
	_, err := s.db.Exec(`
		INSERT INTO signals (symbol, contract_type, detail, strength, window_size, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, sig.Symbol, string(sig.ContractType), sig.Detail, sig.Strength, sig.WindowSize, sig.CreatedAt)
	return err
}

// RecentSignalRow is a row returned by RecentSignals.
type RecentSignalRow struct {
	Symbol       string    `json:"symbol"`
	ContractType string    `json:"contract_type"`
	Detail       string    `json:"detail"`
	Strength     float64   `json:"strength"`
	WindowSize   int       `json:"window_size"`
	CreatedAt    time.Time `json:"created_at"`
}

// RecentSignals returns the most recent signals, newest first, optionally
// filtered by symbol (pass "" for all symbols).
func (s *Store) RecentSignals(symbol string, limit int) ([]RecentSignalRow, error) {
	var rows *sql.Rows
	var err error
	if symbol == "" {
		rows, err = s.db.Query(`
			SELECT symbol, contract_type, detail, strength, window_size, created_at
			FROM signals ORDER BY created_at DESC LIMIT $1
		`, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT symbol, contract_type, detail, strength, window_size, created_at
			FROM signals WHERE symbol = $1 ORDER BY created_at DESC LIMIT $2
		`, symbol, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RecentSignalRow
	for rows.Next() {
		var r RecentSignalRow
		if err := rows.Scan(&r.Symbol, &r.ContractType, &r.Detail, &r.Strength, &r.WindowSize, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
