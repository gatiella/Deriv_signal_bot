// Package config loads all runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds every tunable value the app needs at startup.
type Config struct {
	// Deriv application registration (see README "Connecting your Deriv account").
	AppID         string
	OAuthLoginURL string // base URL for Deriv's OAuth login redirect
	OAuthRedirect string // must exactly match the redirect URI registered with Deriv
	DerivWSURL    string // Deriv websocket endpoint (public + authorized calls)

	// Storage.
	DatabaseURL string

	// Security.
	AppSecretKeyB64 string // 32-byte AES-256 key, base64-encoded, used to encrypt stored Deriv tokens

	// HTTP server.
	HTTPAddr string

	// Signal engine tuning (all have sane defaults, override via env if you want to experiment).
	DigitWindowSize    int     // how many recent ticks to keep per symbol for digit-frequency stats
	DigitThresholdPct  float64 // percentage-point deviation from the 10% baseline that counts as "hot"/"cold"
	MomentumWindowSize int     // how many recent ticks to look at for the rise/fall momentum indicator
	MomentumThreshold  float64 // fraction (0-1) of ticks moving one direction that counts as a momentum lean
}

// Load reads configuration from environment variables, applying defaults for
// anything not explicitly set. It does not validate secrets are non-empty;
// callers should check the fields they need before using them.
func Load() (*Config, error) {
	cfg := &Config{
		AppID:              getEnv("APP_ID", ""),
		OAuthLoginURL:      getEnv("DERIV_OAUTH_LOGIN_URL", "https://oauth.deriv.com/oauth2/authorize"),
		OAuthRedirect:      getEnv("OAUTH_REDIRECT_URL", "http://localhost:8080/auth/deriv/callback"),
		DerivWSURL:         getEnv("DERIV_WS_URL", "wss://ws.derivws.com/websockets/v3"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		AppSecretKeyB64:    getEnv("APP_SECRET_KEY", ""),
		HTTPAddr:           getEnv("HTTP_ADDR", ":"+getEnv("PORT", "8080")),
		DigitWindowSize:    getEnvInt("DIGIT_WINDOW_SIZE", 200),
		DigitThresholdPct:  getEnvFloat("DIGIT_THRESHOLD_PCT", 4.0),
		MomentumWindowSize: getEnvInt("MOMENTUM_WINDOW_SIZE", 20),
		MomentumThreshold:  getEnvFloat("MOMENTUM_THRESHOLD", 0.65),
	}

	if cfg.AppID == "" {
		return nil, fmt.Errorf("APP_ID is required (register a free application at https://app.deriv.com/account/api-token or the API dashboard)")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required, e.g. postgres://user:pass@localhost:5432/deriv_signal_bot?sslmode=disable")
	}
	if len(cfg.AppSecretKeyB64) == 0 {
		return nil, fmt.Errorf("APP_SECRET_KEY is required: a base64-encoded 32-byte key used to encrypt stored Deriv tokens (see README to generate one)")
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