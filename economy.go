package main

import (
	"database/sql"
	"log"
	"math"
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
	marketPressure       float64
	priceFloor           int
	calibration          CalibrationParams
}

var economy = &EconomyState{
	globalCoinPool:       0,
	globalStarsPurchased: 0,
	dailyEmissionTarget:  1000,
	emissionRemainder:    0,
	marketPressure:       1.0,
	priceFloor:           0,
	calibration: CalibrationParams{
		SeasonID:                     defaultSeasonID,
		P0:                           10,
		CBase:                        1000,
		Alpha:                        3.0,
		SScale:                       25.0,
		GScale:                       1000.0,
		Beta:                         2.6,
		Gamma:                        0.08,
		DailyLoginReward:             20,
		DailyLoginCooldownHours:      20,
		ActivityReward:               3,
		ActivityCooldownSeconds:      300,
		DailyCapEarly:                100,
		DailyCapLate:                 30,
		PassiveActiveIntervalSeconds: 60,
		PassiveIdleIntervalSeconds:   240,
		PassiveActiveAmount:          2,
		PassiveIdleAmount:            1,
		HopeThreshold:                0.22,
	},
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
			coins_distributed,
			emission_remainder,
			market_pressure,
			price_floor,
			last_updated
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (season_id)
		DO UPDATE SET
			global_coin_pool = EXCLUDED.global_coin_pool,
			global_stars_purchased = EXCLUDED.global_stars_purchased,
			coins_distributed = EXCLUDED.coins_distributed,
			emission_remainder = EXCLUDED.emission_remainder,
			market_pressure = EXCLUDED.market_pressure,
			price_floor = EXCLUDED.price_floor,
			last_updated = NOW()
	`,
		seasonID,
		e.globalCoinPool,
		e.globalStarsPurchased,
		e.coinsDistributed,
		e.emissionRemainder,
		e.marketPressure,
		e.priceFloor,
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
	return e.coinsDistributed, e.globalStarsPurchased, e.coinsDistributed
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
		SELECT global_coin_pool, global_stars_purchased, coins_distributed, emission_remainder,
			COALESCE(market_pressure, 1.0), COALESCE(price_floor, 0)
		FROM season_economy
		WHERE season_id = $1
	`, seasonID)

	var pool int64
	var stars int64
	var distributed int64
	var remainder float64
	var pressure float64
	var floor int64

	err := row.Scan(&pool, &stars, &distributed, &remainder, &pressure, &floor)
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
	e.marketPressure = pressure
	e.priceFloor = int(floor)

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
			coins_distributed BIGINT NOT NULL,
			emission_remainder DOUBLE PRECISION NOT NULL,
			market_pressure DOUBLE PRECISION NOT NULL DEFAULT 1.0,
			price_floor BIGINT NOT NULL DEFAULT 0,
			last_updated TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE season_economy
			ADD COLUMN IF NOT EXISTS market_pressure DOUBLE PRECISION NOT NULL DEFAULT 1.0;
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		ALTER TABLE season_economy
			ADD COLUMN IF NOT EXISTS price_floor BIGINT NOT NULL DEFAULT 0;
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		ALTER TABLE season_economy
			ADD COLUMN IF NOT EXISTS coins_distributed BIGINT NOT NULL DEFAULT 0;
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

	// 2️⃣b accounts table
	_, err = db.Exec(`
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
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE accounts
		ADD COLUMN IF NOT EXISTS admin_key_hash TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE accounts
		ADD COLUMN IF NOT EXISTS email TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE accounts
			ADD COLUMN IF NOT EXISTS bio TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE accounts
			ADD COLUMN IF NOT EXISTS pronouns TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE accounts
			ADD COLUMN IF NOT EXISTS location TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE accounts
			ADD COLUMN IF NOT EXISTS website TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE accounts
			ADD COLUMN IF NOT EXISTS avatar_url TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE accounts
		ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';
	`)
	if err != nil {
		return err
	}

	// 2️⃣c sessions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			session_id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_sessions_account_id
		ON sessions (account_id);
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
		ADD COLUMN IF NOT EXISTS daily_earn_total BIGINT NOT NULL DEFAULT 0;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE players
		ADD COLUMN IF NOT EXISTS last_earn_reset_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE players
		ADD COLUMN IF NOT EXISTS drip_multiplier DOUBLE PRECISION NOT NULL DEFAULT 1.0;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE players
		ADD COLUMN IF NOT EXISTS drip_paused BOOLEAN NOT NULL DEFAULT FALSE;
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

	_, err = db.Exec(`
		ALTER TABLE players
		ADD COLUMN IF NOT EXISTS is_bot BOOLEAN NOT NULL DEFAULT FALSE;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE players
		ADD COLUMN IF NOT EXISTS bot_profile TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE players
		ADD COLUMN IF NOT EXISTS created_by TEXT NOT NULL DEFAULT 'human';
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
		CREATE TABLE IF NOT EXISTS ip_whitelist (
			ip TEXT PRIMARY KEY,
			max_accounts INT NOT NULL DEFAULT 2,
			created_at TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
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
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
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
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
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
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE refresh_tokens
			ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE refresh_tokens
			ADD COLUMN IF NOT EXISTS account_id TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'refresh_tokens'
				  AND column_name = 'account_id'
				  AND data_type = 'uuid'
			) THEN
				ALTER TABLE refresh_tokens
					ALTER COLUMN account_id TYPE TEXT
					USING account_id::text;
			END IF;
		END $$;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE refresh_tokens
			ADD COLUMN IF NOT EXISTS token_hash TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE refresh_tokens
			ADD COLUMN IF NOT EXISTS issued_at TIMESTAMPTZ;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE refresh_tokens
			ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE refresh_tokens
			ADD COLUMN IF NOT EXISTS user_agent TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE refresh_tokens
			ADD COLUMN IF NOT EXISTS ip TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE refresh_tokens
			ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT 'auth';
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash
		ON refresh_tokens (token_hash);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_account_id
		ON refresh_tokens (account_id, revoked_at);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		ALTER TABLE notifications
			ADD COLUMN IF NOT EXISTS link TEXT;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS notification_reads (
			notification_id BIGINT NOT NULL,
			account_id TEXT NOT NULL,
			read_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (notification_id, account_id)
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS global_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS password_resets (
			reset_id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL
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

	_, err = db.Exec(`
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
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
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

	// 🔟 player_telemetry table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS player_telemetry (
			id BIGSERIAL PRIMARY KEY,
			account_id TEXT,
			player_id TEXT,
			event_type TEXT NOT NULL,
			payload JSONB,
			created_at TIMESTAMPTZ NOT NULL
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
	return int64(e.coinsDistributed)
}

func (e *EconomyState) Calibration() CalibrationParams {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calibration
}

func (e *EconomyState) SetCalibration(params CalibrationParams) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calibration = params
	e.dailyEmissionTarget = params.CBase
	if e.priceFloor < params.P0 {
		e.priceFloor = params.P0
	}
}

func (e *EconomyState) MarketPressure() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.marketPressure
}

func (e *EconomyState) UpdateMarketPressure(target float64, maxDelta float64) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	if target < 0.6 {
		target = 0.6
	}
	if target > 1.8 {
		target = 1.8
	}
	delta := target - e.marketPressure
	if delta > maxDelta {
		delta = maxDelta
	}
	if delta < -maxDelta {
		delta = -maxDelta
	}
	e.marketPressure += delta
	if e.marketPressure < 0.6 {
		e.marketPressure = 0.6
	}
	return e.marketPressure
}

func (e *EconomyState) ApplyPriceFloor(price int) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if price < e.priceFloor {
		return e.priceFloor
	}
	e.priceFloor = price
	return price
}

func (e *EconomyState) EmissionPerMinute() float64 {
	// dailyEmissionTarget is coins per day
	return float64(e.dailyEmissionTarget) / (24 * 60)
}

func (e *EconomyState) EffectiveDailyEmissionTarget(secondsRemaining int64, coinsInCirculation int64) int {
	params := economy.Calibration()
	return EffectiveDailyEmissionTargetForParams(params, secondsRemaining, coinsInCirculation)
}

func (e *EconomyState) EffectiveEmissionPerMinute(secondsRemaining int64, coinsInCirculation int64) float64 {
	return float64(e.EffectiveDailyEmissionTarget(secondsRemaining, coinsInCirculation)) / (24 * 60)
}

func (e *EconomyState) DailyEmissionTarget() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dailyEmissionTarget
}

func (e *EconomyState) SetDailyEmissionTarget(target int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dailyEmissionTarget = target
}

func ComputeStarPrice(
	coinsInCirculation int64,
	secondsRemaining int64,
) int {
	return ComputeStarPriceWithStars(economy.StarsPurchased(), coinsInCirculation, secondsRemaining)
}

func ComputeStarPriceWithStars(
	starsPurchased int,
	coinsInCirculation int64,
	secondsRemaining int64,
) int {
	params := economy.Calibration()
	price := ComputeStarPriceRaw(params, starsPurchased, coinsInCirculation, secondsRemaining, economy.MarketPressure())
	return economy.ApplyPriceFloor(price)
}

func ComputeStarPriceRaw(
	params CalibrationParams,
	starsPurchased int,
	coinsInCirculation int64,
	secondsRemaining int64,
	marketPressure float64,
) int {
	const seasonSeconds = 28 * 24 * 3600
	progress := 1 - (float64(secondsRemaining) / float64(seasonSeconds))
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	scarcityMultiplier := 1 + (float64(starsPurchased) / params.SScale)
	coinMultiplier := 1 + (float64(coinsInCirculation) / params.GScale)
	timeMultiplier := 1 + params.Alpha*math.Pow(progress, 2)

	lateSpike := 1.0
	if progress > 0.75 {
		lateProgress := (progress - 0.75) / 0.25
		if lateProgress < 0 {
			lateProgress = 0
		}
		if lateProgress > 1 {
			lateProgress = 1
		}
		lateSpike = 1 + 0.6*math.Pow(lateProgress, params.Beta)
	}

	if marketPressure < 0.6 {
		marketPressure = 0.6
	}
	if marketPressure > 1.8 {
		marketPressure = 1.8
	}

	price :=
		float64(params.P0) *
			scarcityMultiplier *
			coinMultiplier *
			timeMultiplier *
			lateSpike *
			marketPressure

	return int(price + 0.9999)
}

func EffectiveDailyEmissionTargetForParams(params CalibrationParams, secondsRemaining int64, coinsInCirculation int64) int {
	const seasonSeconds = 28 * 24 * 3600
	if seasonSeconds <= 0 {
		return params.CBase
	}
	progress := 1 - (float64(secondsRemaining) / float64(seasonSeconds))
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	timeMultiplier := 1 - (0.75 * progress)
	if timeMultiplier < 0.12 {
		timeMultiplier = 0.12
	}

	circulationScale := params.GScale * 4.0
	if circulationScale < 2000 {
		circulationScale = 2000
	}
	coinMultiplier := 1 / (1 + (float64(coinsInCirculation) / circulationScale))
	if coinMultiplier < 0.2 {
		coinMultiplier = 0.2
	}

	effective := int(float64(params.CBase)*timeMultiplier*coinMultiplier + 0.5)
	if effective < 0 {
		effective = 0
	}
	return effective
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
