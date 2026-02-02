package main

import (
	"database/sql"
	"log"
	"sync"
	"time"
)

type EconomyState struct {
	mu                   sync.Mutex
	globalCoinPool       int
	coinsDistributed     int
	globalStarsPurchased int
	dailyEmissionTarget  int
	emissionRemainder    float64
}

var economy = &EconomyState{
	globalCoinPool:       0,
	globalStarsPurchased: 0,
	dailyEmissionTarget:  1000,
	emissionRemainder:    0,
}

func (e *EconomyState) emitCoins(amount int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.globalCoinPool += amount
	log.Println("Economy: emitted coins,", amount, "pool now", e.globalCoinPool)
}

func (e *EconomyState) persist(seasonID string, db *sql.DB) {
	e.mu.Lock()
	defer e.mu.Unlock()

	_, err := db.Exec(`
		INSERT INTO season_economy (
			season_id,
			global_coin_pool,
			global_stars_purchased,
			emission_remainder,
			last_updated
		)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (season_id)
		DO UPDATE SET
			global_coin_pool = EXCLUDED.global_coin_pool,
			global_stars_purchased = EXCLUDED.global_stars_purchased,
			emission_remainder = EXCLUDED.emission_remainder,
			last_updated = NOW()
	`,
		seasonID,
		e.globalCoinPool,
		e.globalStarsPurchased,
		e.emissionRemainder,
	)

	if err != nil {
		log.Println("Economy persist error:", err)
	}
}

func (e *EconomyState) IncrementStars() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.globalStarsPurchased++
}

func (e *EconomyState) Snapshot() (int, int, int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.globalCoinPool, e.globalStarsPurchased, e.coinsDistributed
}

func (e *EconomyState) StarsPurchased() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.globalStarsPurchased
}

func (e *EconomyState) load(seasonID string, db *sql.DB) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	row := db.QueryRow(`
		SELECT global_coin_pool, global_stars_purchased, coins_distributed, emission_remainder
		FROM season_economy
		WHERE season_id = $1
	`, seasonID)

	var pool int64
	var stars int64
	var distributed int64
	var remainder float64

	err := row.Scan(&pool, &stars, &distributed, &remainder)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("Economy: no existing state, starting fresh")
			return nil
		}
		return err
	}

	e.globalCoinPool = int(pool)
	e.globalStarsPurchased = int(stars)
	e.coinsDistributed = int(distributed)
	e.emissionRemainder = remainder

	log.Println(
		"Economy: loaded state",
		"coins =", e.globalCoinPool,
		"stars =", e.globalStarsPurchased,
	)
	return nil
}

func ensureSchema(db *sql.DB) error {

	// 1️⃣ season_economy table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS season_economy (
			season_id TEXT PRIMARY KEY,
			global_coin_pool BIGINT NOT NULL,
			global_stars_purchased BIGINT NOT NULL,
			emission_remainder DOUBLE PRECISION NOT NULL,
			last_updated TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	// 2️⃣ players table (ADDED HERE)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS players (
			player_id TEXT PRIMARY KEY,
			coins BIGINT NOT NULL,
			stars BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			last_active_at TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE players
		ADD COLUMN IF NOT EXISTS last_coin_grant_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE players
		ADD COLUMN IF NOT EXISTS burned_coins BIGINT NOT NULL DEFAULT 0;
	`)
	if err != nil {
		return err
	}

	// 3️⃣ player_ip_associations table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS player_ip_associations (
			player_id TEXT NOT NULL,
			ip TEXT NOT NULL,
			first_seen TIMESTAMPTZ NOT NULL,
			last_seen TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (player_id, ip)
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_player_ip_associations_ip
		ON player_ip_associations (ip);
	`)
	if err != nil {
		return err
	}

	// 4️⃣ player_faucet_claims table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS player_faucet_claims (
			player_id TEXT NOT NULL,
			faucet_key TEXT NOT NULL,
			last_claim_at TIMESTAMPTZ NOT NULL,
			claim_count BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (player_id, faucet_key)
		);
	`)
	if err != nil {
		return err
	}

	// 5️⃣ player_star_variants table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS player_star_variants (
			player_id TEXT NOT NULL,
			variant TEXT NOT NULL,
			count BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (player_id, variant)
		);
	`)
	if err != nil {
		return err
	}

	// 6️⃣ player_boosts table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS player_boosts (
			player_id TEXT NOT NULL,
			boost_type TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (player_id, boost_type)
		);
	`)
	if err != nil {
		return err
	}

	// 7️⃣ system_auctions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS system_auctions (
			auction_id TEXT PRIMARY KEY,
			item_key TEXT NOT NULL,
			min_bid BIGINT NOT NULL,
			current_bid BIGINT NOT NULL,
			current_winner TEXT,
			ends_at TIMESTAMPTZ NOT NULL,
			settled_at TIMESTAMPTZ
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS auction_bids (
			auction_id TEXT NOT NULL,
			player_id TEXT NOT NULL,
			bid BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_auction_bids_auction_id
		ON auction_bids (auction_id);
	`)
	if err != nil {
		return err
	}

	// 8️⃣ season_end_snapshots table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS season_end_snapshots (
			season_id TEXT PRIMARY KEY,
			ended_at TIMESTAMPTZ NOT NULL,
			coins_in_circulation BIGINT NOT NULL,
			stars_purchased BIGINT NOT NULL,
			coins_distributed BIGINT NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	// 9️⃣ season_final_rankings table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS season_final_rankings (
			season_id TEXT NOT NULL,
			player_id TEXT NOT NULL,
			stars BIGINT NOT NULL,
			coins BIGINT NOT NULL,
			captured_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (season_id, player_id)
		);
	`)
	if err != nil {
		return err
	}

	return nil
}

func (e *EconomyState) CoinsInCirculation() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return int64(e.globalCoinPool)
}

func (e *EconomyState) EmissionPerMinute() float64 {
	// dailyEmissionTarget is coins per day
	return float64(e.dailyEmissionTarget) / (24 * 60)
}

func ComputeStarPrice(
	coinsInCirculation int64,
	secondsRemaining int64,
) int {

	const (
		BASE_STAR_PRICE      = 10
		STAR_SCARCITY_SCALE  = 25.0
		COIN_INFLATION_SCALE = 1000.0
		SEASON_SECONDS       = 28 * 24 * 3600
	)

	// 1. Star scarcity (PRIMARY driver)
	starsPurchased := float64(economy.StarsPurchased())
	scarcityMultiplier := 1 + (starsPurchased / STAR_SCARCITY_SCALE)

	// 2. Coin inflation (SECONDARY driver)
	coinMultiplier := 1 + (float64(coinsInCirculation) / COIN_INFLATION_SCALE)

	// 3. Time pressure (AMPLIFIER)
	progress := 1 - (float64(secondsRemaining) / float64(SEASON_SECONDS))
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	timeMultiplier := 1 + progress

	price :=
		float64(BASE_STAR_PRICE) *
			scarcityMultiplier *
			coinMultiplier *
			timeMultiplier

	// ceil without math.Ceil
	return int(price + 0.9999)
}

func (e *EconomyState) AvailableCoins() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.globalCoinPool - e.coinsDistributed
}

// TryDistributeCoins attempts to give coins to players,
// enforcing the emission cap.
// Returns true if successful.
func (e *EconomyState) TryDistributeCoins(amount int) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	available := e.globalCoinPool - e.coinsDistributed
	if available < amount {
		return false
	}

	e.coinsDistributed += amount
	return true
}

// CanDrip returns true if at least 60 seconds have passed
func CanDrip(last time.Time, now time.Time) bool {
	return now.Sub(last) >= time.Minute
}
