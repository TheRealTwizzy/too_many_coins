package main

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"
)

var errDailyCapReached = errors.New("daily cap reached")

const loginSafeguardCooldown = 2 * time.Minute

func seasonProgress(now time.Time) float64 {
	seasonSeconds := seasonLength().Seconds()
	if seasonSeconds <= 0 {
		return 0
	}
	progress := 1 - (seasonEnd().Sub(now).Seconds() / seasonSeconds)
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	return progress
}

func DailyEarnCap(now time.Time) int {
	params := economy.Calibration()
	progress := seasonProgress(now)
	return DailyEarnCapForParams(params, progress)
}

func DailyEarnCapForParams(params CalibrationParams, progress float64) int {
	decay := math.Pow(progress, 1.1)
	cap := float64(params.DailyCapEarly) - (float64(params.DailyCapEarly-params.DailyCapLate) * decay)
	if cap < float64(params.DailyCapLate) {
		cap = float64(params.DailyCapLate)
	}
	capMultiplier := 1.2 - (0.4 * progress)
	if capMultiplier < 0.8 {
		capMultiplier = 0.8
	} else if capMultiplier > 1.2 {
		capMultiplier = 1.2
	}
	cap = cap * capMultiplier
	if cap < float64(params.DailyCapLate) {
		cap = float64(params.DailyCapLate)
	}
	return int(cap + 0.5)
}

func resetDailyEarnIfNeeded(db *sql.DB, playerID string, now time.Time) error {
	var lastReset time.Time
	if err := db.QueryRow(`
		SELECT last_earn_reset_at
		FROM players
		WHERE player_id = $1
	`, playerID).Scan(&lastReset); err != nil {
		return err
	}
	if seasonDayIndex(lastReset) == seasonDayIndex(now) {
		return nil
	}
	_, err := db.Exec(`
		UPDATE players
		SET daily_earn_total = 0,
			last_earn_reset_at = $2
		WHERE player_id = $1
	`, playerID, now)
	return err
}

func RemainingDailyCap(db *sql.DB, playerID string, now time.Time) (int, error) {
	if err := resetDailyEarnIfNeeded(db, playerID, now); err != nil {
		return 0, err
	}
	var currentTotal int64
	if err := db.QueryRow(`
		SELECT daily_earn_total
		FROM players
		WHERE player_id = $1
	`, playerID).Scan(&currentTotal); err != nil {
		return 0, err
	}
	cap := DailyEarnCap(now)
	remaining := cap - int(currentTotal)
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

func seasonDayIndex(t time.Time) int {
	start := seasonStart().UTC()
	if t.Before(start) {
		return 0
	}
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	current := t.UTC()
	currentDay := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, time.UTC)
	index := int(currentDay.Sub(startDay).Hours() / 24)
	if index < 0 {
		return 0
	}
	return index
}

func GrantCoinsWithCap(db *sql.DB, playerID string, amount int, now time.Time, sourceType string, accountID *string) (int, int, error) {
	if amount <= 0 {
		return 0, 0, nil
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	var coinsBefore int64
	var currentTotal int64
	var lastReset time.Time
	if err := tx.QueryRow(`
		SELECT coins, daily_earn_total, last_earn_reset_at
		FROM players
		WHERE player_id = $1
		FOR UPDATE
	`, playerID).Scan(&coinsBefore, &currentTotal, &lastReset); err != nil {
		return 0, 0, err
	}

	if seasonDayIndex(lastReset) != seasonDayIndex(now) {
		currentTotal = 0
		if _, err := tx.Exec(`
			UPDATE players
			SET daily_earn_total = 0,
				last_earn_reset_at = $2
			WHERE player_id = $1
		`, playerID, now); err != nil {
			return 0, 0, err
		}
	}

	cap := DailyEarnCap(now)
	remaining := cap - int(currentTotal)
	if remaining <= 0 {
		return 0, 0, errDailyCapReached
	}

	grant := amount
	if grant > remaining {
		grant = remaining
	}

	coinsAfter := coinsBefore + int64(grant)
	_, err = tx.Exec(`
		UPDATE players
		SET coins = coins + $2,
			daily_earn_total = daily_earn_total + $2,
			last_coin_grant_at = $3
		WHERE player_id = $1
	`, playerID, grant, now)
	if err != nil {
		return 0, remaining, err
	}

	if grant > 0 {
		var accountValue sql.NullString
		if accountID != nil && *accountID != "" {
			accountValue = sql.NullString{String: *accountID, Valid: true}
		}
		if _, err := tx.Exec(`
			INSERT INTO coin_earning_log (
				account_id,
				player_id,
				season_id,
				source_type,
				amount,
				coins_before,
				coins_after,
				created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, accountValue, playerID, currentSeasonID(), sourceType, grant, coinsBefore, coinsAfter, now); err != nil {
			return 0, remaining, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, remaining, err
	}

	return grant, remaining - grant, nil
}

func GrantCoinsNoCap(db *sql.DB, playerID string, amount int, now time.Time, sourceType string, accountID *string) (int, error) {
	if amount <= 0 {
		return 0, nil
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var coinsBefore int64
	if err := tx.QueryRow(`
		SELECT coins
		FROM players
		WHERE player_id = $1
		FOR UPDATE
	`, playerID).Scan(&coinsBefore); err != nil {
		return 0, err
	}

	coinsAfter := coinsBefore + int64(amount)
	if _, err := tx.Exec(`
		UPDATE players
		SET coins = coins + $2,
			last_coin_grant_at = $3
		WHERE player_id = $1
	`, playerID, amount, now); err != nil {
		return 0, err
	}

	var accountValue sql.NullString
	if accountID != nil && *accountID != "" {
		accountValue = sql.NullString{String: *accountID, Valid: true}
	}
	if _, err := tx.Exec(`
		INSERT INTO coin_earning_log (
			account_id,
			player_id,
			season_id,
			source_type,
			amount,
			coins_before,
			coins_after,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, accountValue, playerID, currentSeasonID(), sourceType, amount, coinsBefore, coinsAfter, now); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return amount, nil
}

func EnsurePlayableBalanceOnLogin(db *sql.DB, playerID string, accountID *string) {
	now := time.Now().UTC()
	if isSeasonEnded(now) {
		return
	}
	cooldown := loginSafeguardCooldown

	var lastGrant sql.NullTime
	if err := db.QueryRow(`
		SELECT MAX(created_at)
		FROM coin_earning_log
		WHERE player_id = $1 AND source_type = $2
	`, playerID, FaucetLogin).Scan(&lastGrant); err == nil {
		if lastGrant.Valid && now.Sub(lastGrant.Time) < cooldown {
			return
		}
	}

	var coins int64
	if err := db.QueryRow(`
		SELECT coins
		FROM players
		WHERE player_id = $1
	`, playerID).Scan(&coins); err != nil {
		return
	}

	params := economy.Calibration()
	coinsInCirculation := economy.CoinsInCirculation()
	secondsRemaining := seasonSecondsRemaining(now)
	currentPrice := ComputeSeasonAuthorityStarPrice(coinsInCirculation, secondsRemaining)
	buffer := maxInt(1, params.ActivityReward)
	minBalance := currentPrice + buffer
	if minBalance < params.P0 {
		minBalance = params.P0
	}
	if minBalance < params.DailyLoginReward {
		minBalance = params.DailyLoginReward
	}
	if coins >= int64(minBalance) {
		return
	}

	needed := minBalance - int(coins)
	if !economy.TryDistributeCoins(needed) {
		emitServerTelemetryWithCooldown(db, accountID, playerID, "login_safeguard_denied_emission", map[string]interface{}{
			"needed":           needed,
			"availableCoins":   economy.AvailableCoins(),
			"minBalance":       minBalance,
			"currentCoins":     coins,
			"starPrice":        currentPrice,
			"buffer":           buffer,
			"p0":               params.P0,
			"dailyLoginReward": params.DailyLoginReward,
			"cooldownSeconds":  int(cooldown.Seconds()),
		}, 5*time.Minute)
		return
	}
	_, _ = GrantCoinsNoCap(db, playerID, needed, now, FaucetLogin, accountID)
	emitServerTelemetryWithCooldown(db, accountID, playerID, "login_safeguard_triggered", map[string]interface{}{
		"granted":          needed,
		"minBalance":       minBalance,
		"currentCoins":     coins,
		"starPrice":        currentPrice,
		"buffer":           buffer,
		"p0":               params.P0,
		"dailyLoginReward": params.DailyLoginReward,
		"cooldownSeconds":  int(cooldown.Seconds()),
	}, 5*time.Minute)
}
