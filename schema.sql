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

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS trust_status TEXT NOT NULL DEFAULT 'normal';

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

CREATE TABLE IF NOT EXISTS auth_rate_limits (
    ip TEXT NOT NULL,
    action TEXT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    attempt_count INT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (ip, action)
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_account_id
    ON refresh_tokens (account_id, revoked_at);

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS link TEXT;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS recipient_role TEXT;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS recipient_account_id TEXT;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS season_id TEXT;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS category TEXT;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS type TEXT;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS priority TEXT NOT NULL DEFAULT 'normal';

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS payload JSONB;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS ack_required BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMPTZ;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS dedupe_key TEXT;

CREATE TABLE IF NOT EXISTS notification_reads (
    notification_id BIGINT NOT NULL,
    account_id TEXT NOT NULL,
    read_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (notification_id, account_id)
);

CREATE TABLE IF NOT EXISTS notification_acks (
    notification_id BIGINT NOT NULL,
    account_id TEXT NOT NULL,
    acknowledged_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (notification_id, account_id)
);

CREATE TABLE IF NOT EXISTS notification_deletes (
    notification_id BIGINT NOT NULL,
    account_id TEXT NOT NULL,
    deleted_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (notification_id, account_id)
);

CREATE TABLE IF NOT EXISTS notification_settings (
    account_id TEXT NOT NULL,
    category TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    push_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (account_id, category)
);

ALTER TABLE notification_settings
    ADD COLUMN IF NOT EXISTS push_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_notifications_created_at
    ON notifications (created_at);

CREATE INDEX IF NOT EXISTS idx_notifications_dedupe
    ON notifications (dedupe_key, created_at);

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

CREATE TABLE IF NOT EXISTS coin_earning_log (
    id BIGSERIAL PRIMARY KEY,
    account_id TEXT,
    player_id TEXT NOT NULL,
    season_id TEXT NOT NULL,
    source_type TEXT NOT NULL,
    amount BIGINT NOT NULL,
    coins_before BIGINT NOT NULL,
    coins_after BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
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

CREATE TABLE IF NOT EXISTS player_abuse_state (
    player_id TEXT NOT NULL,
    season_id TEXT NOT NULL,
    score DOUBLE PRECISION NOT NULL DEFAULT 0,
    severity INT NOT NULL DEFAULT 0,
    last_signal_at TIMESTAMPTZ,
    last_decay_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    persistent_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (player_id, season_id)
);

CREATE TABLE IF NOT EXISTS account_abuse_reputation (
    account_id TEXT PRIMARY KEY,
    score DOUBLE PRECISION NOT NULL DEFAULT 0,
    severity INT NOT NULL DEFAULT 0,
    last_signal_at TIMESTAMPTZ,
    last_decay_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    persistent_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS abuse_events (
    id BIGSERIAL PRIMARY KEY,
    account_id TEXT,
    player_id TEXT,
    season_id TEXT,
    event_type TEXT NOT NULL,
    severity INT NOT NULL,
    score_delta DOUBLE PRECISION NOT NULL DEFAULT 0,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_abuse_events_created_at
    ON abuse_events (created_at);

CREATE INDEX IF NOT EXISTS idx_player_abuse_state_score
    ON player_abuse_state (season_id, score);

