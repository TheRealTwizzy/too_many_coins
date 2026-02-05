-- ============================================================
-- Too Many Coins — Canonical Schema
-- Phase 0: Infrastructure Alignment
--
-- IMPORTANT:
-- Tables marked PHASE 0 REQUIRED are authoritative now.
-- Tables marked POST-ALPHA exist but MUST NOT be assumed active.
-- No Alpha code paths should rely on POST-ALPHA tables.
-- ============================================================

-- =========================
-- PHASE 0 REQUIRED
-- Core season-wide economy state (single season in Alpha)
-- =========================
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

-- =========================
-- PHASE 0 REQUIRED
-- Player core state (authoritative balances + activity)
-- =========================
CREATE TABLE IF NOT EXISTS players (
    player_id TEXT PRIMARY KEY,
    coins BIGINT NOT NULL,
    stars BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    last_active_at TIMESTAMPTZ NOT NULL
);

-- =========================
-- PHASE 0 REQUIRED
-- Account identity and authentication
-- =========================
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

ALTER TABLE accounts ADD COLUMN IF NOT EXISTS admin_key_hash TEXT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS bio TEXT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS pronouns TEXT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS location TEXT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS website TEXT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS avatar_url TEXT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS trust_status TEXT NOT NULL DEFAULT 'normal';
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE;

-- =========================
-- PHASE 0 REQUIRED
-- Session persistence (HTTP-only cookies)
-- =========================
CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_account_id
    ON sessions (account_id);

-- =========================
-- PHASE 0 REQUIRED
-- Player earning & activity tracking (even if faucets disabled)
-- =========================
ALTER TABLE players ADD COLUMN IF NOT EXISTS last_coin_grant_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE players ADD COLUMN IF NOT EXISTS daily_earn_total BIGINT NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN IF NOT EXISTS last_earn_reset_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- =========================
-- POST-ALPHA / DISABLED IN PHASE 0
-- Passive drip & advanced modifiers
-- =========================
ALTER TABLE players ADD COLUMN IF NOT EXISTS drip_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1.0;
ALTER TABLE players ADD COLUMN IF NOT EXISTS drip_paused BOOLEAN NOT NULL DEFAULT FALSE;

-- =========================
-- PHASE 0 REQUIRED
-- Burn tracking, bots, provenance
-- =========================
ALTER TABLE players ADD COLUMN IF NOT EXISTS burned_coins BIGINT NOT NULL DEFAULT 0;
ALTER TABLE players ADD COLUMN IF NOT EXISTS is_bot BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE players ADD COLUMN IF NOT EXISTS bot_profile TEXT;
ALTER TABLE players ADD COLUMN IF NOT EXISTS created_by TEXT NOT NULL DEFAULT 'human';

-- =========================
-- PHASE 0 REQUIRED
-- IP association & abuse groundwork
-- =========================
CREATE TABLE IF NOT EXISTS player_ip_associations (
    player_id TEXT NOT NULL,
    ip TEXT NOT NULL,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (player_id, ip)
);

CREATE INDEX IF NOT EXISTS idx_player_ip_associations_ip
    ON player_ip_associations (ip);

-- =========================
-- PHASE 0 REQUIRED
-- Notifications (delivery + audit only)
-- =========================
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

ALTER TABLE notifications ADD COLUMN IF NOT EXISTS recipient_role TEXT;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS recipient_account_id TEXT;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS season_id TEXT;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS category TEXT;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS type TEXT;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS priority TEXT NOT NULL DEFAULT 'normal';
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS payload JSONB;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS ack_required BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMPTZ;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS dedupe_key TEXT;

CREATE INDEX IF NOT EXISTS idx_notifications_created_at
    ON notifications (created_at);

CREATE INDEX IF NOT EXISTS idx_notifications_dedupe
    ON notifications (dedupe_key, created_at);

-- =========================
-- PHASE 0 REQUIRED
-- Notification read / ack state
-- =========================
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

-- =========================
-- PHASE 0 REQUIRED
-- Admin bootstrap & auth safety
-- =========================
CREATE TABLE IF NOT EXISTS admin_bootstrap_tokens (
    token_hash TEXT PRIMARY KEY,
    used_at TIMESTAMPTZ,
    used_by_account_id TEXT,
    used_by_ip TEXT
);

CREATE TABLE IF NOT EXISTS admin_password_gates (
    gate_id BIGSERIAL PRIMARY KEY,
    account_id TEXT NOT NULL,
    gate_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    used_by_ip TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_password_gates_active
    ON admin_password_gates (account_id)
    WHERE used_at IS NULL;

-- =========================
-- PHASE 0 REQUIRED
-- Auth rate limiting
-- =========================
CREATE TABLE IF NOT EXISTS auth_rate_limits (
    ip TEXT NOT NULL,
    action TEXT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    attempt_count INT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (ip, action)
);

-- =========================
-- PHASE 0 REQUIRED
-- Password reset flow
-- =========================
CREATE TABLE IF NOT EXISTS password_resets (
    reset_id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

-- =========================
-- POST-ALPHA / ECONOMY DETAIL
-- Logs, calibration, ranking, telemetry
-- =========================
-- NOTE: Present for continuity; not all are active in Phase 0

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

CREATE TABLE IF NOT EXISTS season_calibration (
    season_id TEXT PRIMARY KEY,
    seed BIGINT NOT NULL,
    p0 INT NOT NULL,
    c_base INT NOT NULL,
    alpha DOUBLE PRECISION NOT NULL,
    s_scale DOUBLE PRECISION NOT NULL,
    g_scale DOUBLE PRECISION NOT NULL,
    beta DOUBLE PRECISION NOT NULL,
    gamma DOUBLE PRECISION NOT NULL,
    daily_login_reward INT NOT NULL,
    daily_login_cooldown_hours INT NOT NULL,
    activity_reward INT NOT NULL,
    activity_cooldown_seconds INT NOT NULL,
    daily_cap_early INT NOT NULL,
    daily_cap_late INT NOT NULL,
    passive_active_interval_seconds INT NOT NULL,
    passive_idle_interval_seconds INT NOT NULL,
    passive_active_amount INT NOT NULL,
    passive_idle_amount INT NOT NULL,
    hope_threshold DOUBLE PRECISION NOT NULL DEFAULT 0.22,
    created_at TIMESTAMPTZ NOT NULL
);

-- =========================
-- POST-ALPHA / BETA-ONLY
-- Tradable Seasonal Assets (TSA)
-- =========================
-- No Alpha code paths may reference these tables.

CREATE TABLE IF NOT EXISTS tsa_cinder_sigils (
    sigil_id TEXT PRIMARY KEY,
    season_id TEXT NOT NULL,
    minted_at TIMESTAMPTZ NOT NULL,
    minted_day INT NOT NULL,
    owner_player_id TEXT,
    owner_account_id TEXT,
    status TEXT NOT NULL,
    trade_count INT NOT NULL DEFAULT 0,
    last_trade_at TIMESTAMPTZ,
    last_status_at TIMESTAMPTZ NOT NULL,
    activation_choice TEXT,
    activated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS tsa_mint_log (
    id BIGSERIAL PRIMARY KEY,
    sigil_id TEXT NOT NULL,
    season_id TEXT NOT NULL,
    minted_day INT NOT NULL,
    buyer_player_id TEXT NOT NULL,
    buyer_account_id TEXT,
    price_paid BIGINT NOT NULL,
    coins_before BIGINT NOT NULL,
    coins_after BIGINT NOT NULL,
    active_players INT NOT NULL,
    daily_mint_cap INT NOT NULL,
    season_mint_cap INT NOT NULL,
    minted_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS tsa_trade_log (
    id BIGSERIAL PRIMARY KEY,
    sigil_id TEXT NOT NULL,
    season_id TEXT NOT NULL,
    seller_player_id TEXT NOT NULL,
    seller_account_id TEXT,
    buyer_player_id TEXT NOT NULL,
    buyer_account_id TEXT,
    price_paid BIGINT NOT NULL,
    burn_amount BIGINT NOT NULL,
    destroyed BOOLEAN NOT NULL,
    trade_status TEXT NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS tsa_activation_log (
    id BIGSERIAL PRIMARY KEY,
    sigil_id TEXT NOT NULL,
    season_id TEXT NOT NULL,
    player_id TEXT NOT NULL,
    account_id TEXT,
    activation_choice TEXT NOT NULL,
    activated_at TIMESTAMPTZ NOT NULL
);
