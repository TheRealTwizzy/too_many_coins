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
