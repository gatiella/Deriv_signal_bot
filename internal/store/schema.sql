-- Schema for deriv-signal-bot (signals-only build - no account linking,
-- see README for why). Applied automatically on startup by internal/store.Open.

CREATE TABLE IF NOT EXISTS signals (
    id            BIGSERIAL PRIMARY KEY,
    symbol        TEXT NOT NULL,
    contract_type TEXT NOT NULL,
    detail        TEXT NOT NULL,
    strength      DOUBLE PRECISION NOT NULL,
    window_size   INT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_signals_created_at ON signals (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_signals_symbol ON signals (symbol);