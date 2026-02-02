CREATE TABLE IF NOT EXISTS season_economy (
    season_id TEXT PRIMARY KEY,
    global_coin_pool BIGINT NOT NULL,
    emission_remainder DOUBLE PRECISION NOT NULL,
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
    created_at TIMESTAMPTZ NOT NULL,
    last_login_at TIMESTAMPTZ NOT NULL
);

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
    ADD COLUMN IF NOT EXISTS burned_coins BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS player_ip_associations (
    player_id TEXT NOT NULL,
    ip TEXT NOT NULL,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (player_id, ip)
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

CREATE TABLE IF NOT EXISTS player_boosts (
    player_id TEXT NOT NULL,
    boost_type TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (player_id, boost_type)
);

CREATE TABLE IF NOT EXISTS system_auctions (
    auction_id TEXT PRIMARY KEY,
    item_key TEXT NOT NULL,
    min_bid BIGINT NOT NULL,
    current_bid BIGINT NOT NULL,
    current_winner TEXT,
    ends_at TIMESTAMPTZ NOT NULL,
    settled_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS auction_bids (
    auction_id TEXT NOT NULL,
    player_id TEXT NOT NULL,
    bid BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auction_bids_auction_id
    ON auction_bids (auction_id);

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

CREATE TABLE IF NOT EXISTS player_feedback (
    id BIGSERIAL PRIMARY KEY,
    account_id TEXT,
    player_id TEXT,
    rating INT,
    message TEXT NOT NULL,
    context JSONB,
    created_at TIMESTAMPTZ NOT NULL
);
