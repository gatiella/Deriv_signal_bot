// Package config loads all runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds every tunable value the app needs at startup.
type Config struct {
	// Deriv application registration - must be a classic app_id valid for
	// wss://ws.derivws.com/websockets/v3 (see README: the newer
	// developers.deriv.com portal issues app_ids for a different, incompatible
	// system - use the shared public test id 1089 until you have your own).
	AppID      string
	DerivWSURL string // Deriv websocket endpoint for public market data

	// Storage.
	DatabaseURL string

	// HTTP server.
	HTTPAddr string

	// Signal engine tuning (all have sane defaults, override via env if you want to experiment).
	DigitWindowSize    int     // how many recent ticks to keep per symbol for digit-frequency stats
	DigitThresholdPct  float64 // percentage-point deviation from the 10% baseline that counts as "hot"/"cold"
	MomentumWindowSize int     // how many recent ticks to look at for the rise/fall momentum indicator
	MomentumThreshold  float64 // fraction (0-1) of ticks moving one direction that counts as a momentum lean
}

// Load reads configuration from environment variables, applying defaults for
// anything not explicitly set.
func Load() (*Config, error) {
	cfg := &Config{
		AppID:              getEnv("APP_ID", ""),
		DerivWSURL:         getEnv("DERIV_WS_URL", "wss://ws.derivws.com/websockets/v3"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		HTTPAddr:           getEnv("HTTP_ADDR", ":"+getEnv("PORT", "8080")),
		DigitWindowSize:    getEnvInt("DIGIT_WINDOW_SIZE", 200),
		DigitThresholdPct:  getEnvFloat("DIGIT_THRESHOLD_PCT", 4.0),
		MomentumWindowSize: getEnvInt("MOMENTUM_WINDOW_SIZE", 20),
		MomentumThreshold:  getEnvFloat("MOMENTUM_THRESHOLD", 0.65),
	}

	if cfg.AppID == "" {
		return nil, fmt.Errorf("APP_ID is required (use 1089 for the shared public test app, or your own classic app_id)")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required, e.g. postgres://user:pass@localhost:5432/deriv_signal_bot?sslmode=disable")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}