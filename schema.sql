CREATE TABLE IF NOT EXISTS season_economy (
    season_id TEXT PRIMARY KEY,
    global_coin_pool BIGINT NOT NULL,
    global_stars_purchased BIGINT NOT NULL,
    coins_distributed BIGINT NOT NULL,
    emission_remainder DOUBLE PRECISION NOT NULL,
    market_pressure DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    price_floor BIGINT NOT NULL DEFAULT 0,
    last_updated TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS players (
    player_id TEXT PRIMARY KEY,
    coins BIGINT NOT NULL,
    stars BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    last_active_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS accounts (
    account_id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL,
    player_id TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL,
    last_login_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS admin_key_hash TEXT;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS email TEXT;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS bio TEXT;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS pronouns TEXT;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS location TEXT;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS website TEXT;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS avatar_url TEXT;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';

CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_account_id
    ON sessions (account_id);

ALTER TABLE players
    ADD COLUMN IF NOT EXISTS last_coin_grant_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE players
    ADD COLUMN IF NOT EXISTS drip_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1.0;

ALTER TABLE players
    ADD COLUMN IF NOT EXISTS drip_paused BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE players
    ADD COLUMN IF NOT EXISTS burned_coins BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS player_ip_associations (
    player_id TEXT NOT NULL,
    ip TEXT NOT NULL,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (player_id, ip)
);

CREATE TABLE IF NOT EXISTS ip_whitelist (
    ip TEXT PRIMARY KEY,
    max_accounts INT NOT NULL DEFAULT 2,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS ip_whitelist_requests (
    request_id TEXT PRIMARY KEY,
    ip TEXT NOT NULL,
    account_id TEXT,
    reason TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    resolved_by TEXT
);

CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    target_role TEXT NOT NULL,
    account_id TEXT,
    message TEXT NOT NULL,
    level TEXT NOT NULL DEFAULT 'info',
    link TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    account_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    user_agent TEXT,
    ip TEXT,
    purpose TEXT NOT NULL DEFAULT 'auth'
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_account_id
    ON refresh_tokens (account_id, revoked_at);

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS link TEXT;

CREATE TABLE IF NOT EXISTS notification_reads (
    notification_id BIGINT NOT NULL,
    account_id TEXT NOT NULL,
    read_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (notification_id, account_id)
);

CREATE TABLE IF NOT EXISTS global_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS password_resets (
    reset_id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_player_ip_associations_ip
    ON player_ip_associations (ip);

CREATE TABLE IF NOT EXISTS player_faucet_claims (
    player_id TEXT NOT NULL,
    faucet_key TEXT NOT NULL,
    last_claim_at TIMESTAMPTZ NOT NULL,
    claim_count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (player_id, faucet_key)
);

CREATE TABLE IF NOT EXISTS player_star_variants (
    player_id TEXT NOT NULL,
    variant TEXT NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (player_id, variant)
);

CREATE TABLE IF NOT EXISTS star_purchase_log (
    id BIGSERIAL PRIMARY KEY,
    account_id TEXT,
    player_id TEXT NOT NULL,
    season_id TEXT NOT NULL,
    purchase_type TEXT NOT NULL,
    variant TEXT,
    price_paid BIGINT NOT NULL,
    coins_before BIGINT NOT NULL,
    coins_after BIGINT NOT NULL,
    stars_before BIGINT NOT NULL,
    stars_after BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS player_boosts (
    player_id TEXT NOT NULL,
    boost_type TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (player_id, boost_type)
);

CREATE TABLE IF NOT EXISTS season_end_snapshots (
    season_id TEXT PRIMARY KEY,
    ended_at TIMESTAMPTZ NOT NULL,
    coins_in_circulation BIGINT NOT NULL,
    stars_purchased BIGINT NOT NULL,
    coins_distributed BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS season_final_rankings (
    season_id TEXT NOT NULL,
    player_id TEXT NOT NULL,
    stars BIGINT NOT NULL,
    coins BIGINT NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (season_id, player_id)
);

CREATE TABLE IF NOT EXISTS player_telemetry (
    id BIGSERIAL PRIMARY KEY,
    account_id TEXT,
    player_id TEXT,
    event_type TEXT NOT NULL,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL
);

