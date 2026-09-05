-- Initial schema for deriv-signal-bot.
-- Applied automatically on startup by internal/store.Migrate (idempotent).

CREATE TABLE IF NOT EXISTS users (
    id         BIGSERIAL PRIMARY KEY,
    email      TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per Deriv account a user has connected (a user may have both a
-- demo and a real account, or multiple currencies).
CREATE TABLE IF NOT EXISTS deriv_accounts (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    loginid       TEXT NOT NULL,
    currency      TEXT,
    is_virtual    BOOLEAN NOT NULL DEFAULT false,
    api_token_enc TEXT NOT NULL, -- AES-256-GCM encrypted, see internal/cryptoutil
    connected_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, loginid)
);

-- Server-side sessions; the browser only holds a random session id cookie.
CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- History of every edge-triggered signal the engine has emitted, so the
-- dashboard can show "recent signals" and you can later analyze which
-- conditions fired most often.
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
