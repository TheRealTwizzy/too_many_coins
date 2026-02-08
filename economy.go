package main

import (
	"database/sql"
	_ "embed"
	"log"
	"math"
	"sync"
	"time"
)

// COIN_SCALE: 1 coin = 1000 microcoins
// All internal economy math uses microcoins
// This allows UBI of 0.001 coins (1 microcoin) per tick to be represented as integers
const COIN_SCALE = 1000

//go:embed schema.sql
var schemaSQL string

type EconomyState struct {
	mu                   sync.Mutex
	globalCoinPool       int
	coinsDistributed     int
	coinsInWallets       int64
	activeCoinsInWallets int64
	activePlayers        int
	globalStarsPurchased int
	dailyEmissionTarget  int
	emissionRemainder    float64
	marketPressure       float64
	priceFloor           int
	currentStarPrice     int
	calibration          CalibrationParams
}

type EconomyInvariantSnapshot struct {
	GlobalCoinPool   int
	CoinsDistributed int
	AvailableCoins   int
	MarketPressure   float64
}

var economy = &EconomyState{
	globalCoinPool:       0,
	globalStarsPurchased: 0,
	dailyEmissionTarget:  1000 * COIN_SCALE, // 1000 coins = 1,000,000 microcoins
	emissionRemainder:    0,
	marketPressure:       1.0,
	priceFloor:           0,
	calibration: CalibrationParams{
		SeasonID:                     defaultSeasonID,
		P0:                           10 * COIN_SCALE,   // 10 coins = 10,000 microcoins
		CBase:                        1000 * COIN_SCALE, // 1000 coins = 1,000,000 microcoins
		Alpha:                        3.0,
		SScale:                       25.0,
		GScale:                       1000.0 * float64(COIN_SCALE),
		Beta:                         2.6,
		Gamma:                        0.08,
		DailyLoginReward:             20 * COIN_SCALE, // 20 coins
		DailyLoginCooldownHours:      20,
		ActivityReward:               3 * COIN_SCALE, // 3 coins
		ActivityCooldownSeconds:      300,
		DailyCapEarly:                100 * COIN_SCALE, // 100 coins
		DailyCapLate:                 30 * COIN_SCALE,  // 30 coins
		PassiveActiveIntervalSeconds: 60,
		PassiveIdleIntervalSeconds:   240,
		PassiveActiveAmount:          2 * COIN_SCALE, // 2 coins
		PassiveIdleAmount:            1 * COIN_SCALE, // 1 coin
		HopeThreshold:                0.22,
	},
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
			current_star_price,
			last_updated
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (season_id)
		DO UPDATE SET
			global_coin_pool = EXCLUDED.global_coin_pool,
			global_stars_purchased = EXCLUDED.global_stars_purchased,
			coins_distributed = EXCLUDED.coins_distributed,
			emission_remainder = EXCLUDED.emission_remainder,
			market_pressure = EXCLUDED.market_pressure,
			price_floor = EXCLUDED.price_floor,
			current_star_price = EXCLUDED.current_star_price,
			last_updated = NOW()
	`,
		seasonID,
		e.globalCoinPool,
		e.globalStarsPurchased,
		e.coinsDistributed,
		e.emissionRemainder,
		e.marketPressure,
		e.priceFloor,
		e.currentStarPrice,
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
	return int(e.coinsInWallets), e.globalStarsPurchased, e.coinsDistributed
}

func (e *EconomyState) StarsPurchased() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.globalStarsPurchased
}

func (e *EconomyState) CurrentStarPrice() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentStarPrice
}

func (e *EconomyState) SetCurrentStarPrice(price int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.currentStarPrice = price
}

func (e *EconomyState) load(seasonID string, db *sql.DB) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	row := db.QueryRow(`
		SELECT global_coin_pool, global_stars_purchased, coins_distributed, emission_remainder,
			COALESCE(market_pressure, 1.0), COALESCE(price_floor, 0), COALESCE(current_star_price, 0)
		FROM season_economy
		WHERE season_id = $1
	`, seasonID)

	var pool int64
	var stars int64
	var distributed int64
	var remainder float64
	var pressure float64
	var floor int64
	var starPrice float64

	err := row.Scan(&pool, &stars, &distributed, &remainder, &pressure, &floor, &starPrice)
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
	e.currentStarPrice = int(starPrice)

	log.Println(
		"Economy: loaded state",
		"coins =", e.globalCoinPool,
		"stars =", e.globalStarsPurchased,
		"star_price =", e.currentStarPrice,
	)
	return nil
}

func ensureSchema(db *sql.DB) error {
	log.Println("Applying schema.sql...")

	if _, err := db.Exec(schemaSQL); err != nil {
		log.Printf("ERROR: schema.sql application failed: %v", err)
		return err
	}

	log.Println("schema.sql applied successfully")
	return nil
}

func EnsureSchema(db *sql.DB) error {
	return ensureSchema(db)
}

func (e *EconomyState) CoinsInCirculation() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.coinsInWallets < 0 {
		return 0
	}
	return e.coinsInWallets
}

func (e *EconomyState) SetCoinsInWallets(total int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if total < 0 {
		total = 0
	}
	e.coinsInWallets = total
}

func (e *EconomyState) SetCirculationStats(total int64, activeCoins int64, activePlayers int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if total < 0 {
		total = 0
	}
	if activeCoins < 0 {
		activeCoins = 0
	}
	if activePlayers < 0 {
		activePlayers = 0
	}
	e.coinsInWallets = total
	e.activeCoinsInWallets = activeCoins
	e.activePlayers = activePlayers
}

func (e *EconomyState) ActiveCoinsInCirculation() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activeCoinsInWallets < 0 {
		return 0
	}
	return e.activeCoinsInWallets
}

func (e *EconomyState) ActivePlayers() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.activePlayers < 0 {
		return 0
	}
	return e.activePlayers
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
	activeCoins := economy.ActiveCoinsInCirculation()
	activePlayers := economy.ActivePlayers()
	price := ComputeStarPriceRawWithActive(params, starsPurchased, coinsInCirculation, activeCoins, activePlayers, secondsRemaining, economy.MarketPressure())
	return economy.ApplyPriceFloor(price)
}

func ComputeSeasonAuthorityStarPrice(
	coinsInCirculation int64,
	secondsRemaining int64,
) int {
	// Season-authoritative star price computation.
	// Uses ONLY season-level inputs; MUST NOT read active player metrics.
	// Inputs:
	// - time progression (secondsRemaining)
	// - market pressure (from season economy state)
	// - late-season spike (derived from time)
	// - affordability guardrail (derived from total coins / expected players)
	// Output:
	// - authoritative star price shared identically by all players
	params := economy.Calibration()
	starsPurchased := economy.StarsPurchased()
	marketPressure := economy.MarketPressure()
	// Call raw computation with activePlayers=0 to bypass active-player-based logic.
	// This ensures all players see the same price.
	price := ComputeStarPriceRaw(params, starsPurchased, coinsInCirculation, secondsRemaining, marketPressure)
	return economy.ApplyPriceFloor(price)
}

func ComputeStarPriceRaw(
	params CalibrationParams,
	starsPurchased int,
	coinsInCirculation int64,
	secondsRemaining int64,
	marketPressure float64,
) int {
	return ComputeStarPriceRawWithActive(params, starsPurchased, coinsInCirculation, coinsInCirculation, 0, secondsRemaining, marketPressure)
}

func ComputeStarPriceRawWithActive(
	params CalibrationParams,
	starsPurchased int,
	coinsInCirculation int64,
	activeCoinsInCirculation int64,
	activePlayers int,
	secondsRemaining int64,
	marketPressure float64,
) int {
	seasonSeconds := seasonLength().Seconds()
	if seasonSeconds <= 0 {
		seasonSeconds = 1
	}
	progress := 1 - (float64(secondsRemaining) / seasonSeconds)
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	scarcityMultiplier := 1 + (float64(starsPurchased) / params.SScale)

	capEarly := float64(params.DailyCapEarly)
	if capEarly <= 0 {
		capEarly = 1
	}
	expectedPlayers := float64(params.CBase) / (capEarly * 0.6)
	if expectedPlayers < 10 {
		expectedPlayers = 10
	}
	coinsPerPlayer := 0.0
	if activePlayers > 0 {
		coinsPerPlayer = float64(activeCoinsInCirculation) / float64(activePlayers)
	} else {
		coinsPerPlayer = float64(coinsInCirculation) / expectedPlayers
	}
	if coinsPerPlayer < 0 {
		coinsPerPlayer = 0
	}
	coinPressure := coinsPerPlayer / capEarly
	if coinPressure < 0 {
		coinPressure = 0
	}
	coinMultiplier := 1 + 0.55*math.Log1p(coinPressure)

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

	affordabilityCap := coinsPerPlayer * 0.9
	if affordabilityCap < float64(params.P0) {
		affordabilityCap = float64(params.P0)
	}
	if price > affordabilityCap {
		price = affordabilityCap
	}

	return int(price + 0.9999)
}

func EffectiveDailyEmissionTargetForParams(params CalibrationParams, secondsRemaining int64, coinsInCirculation int64) int {
	seasonSeconds := seasonLength().Seconds()
	if seasonSeconds <= 0 {
		return params.CBase
	}
	progress := 1 - (float64(secondsRemaining) / seasonSeconds)
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

	minFloor := int(float64(params.CBase)*0.25 + 0.5)
	if minFloor < params.DailyCapLate {
		minFloor = params.DailyCapLate
	}
	if effective < minFloor {
		effective = minFloor
	}
	return effective
}

func (e *EconomyState) AvailableCoins() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.globalCoinPool - e.coinsDistributed
}

func (e *EconomyState) InvariantSnapshot() EconomyInvariantSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	available := e.globalCoinPool - e.coinsDistributed
	return EconomyInvariantSnapshot{
		GlobalCoinPool:   e.globalCoinPool,
		CoinsDistributed: e.coinsDistributed,
		AvailableCoins:   available,
		MarketPressure:   e.marketPressure,
	}
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
