package main

import (
	"database/sql"
	"time"
)

const (
	FaucetPassive  = "passive"
	FaucetDaily    = "daily"
	FaucetActivity = "activity"
	FaucetLogin    = "login"
)

func CanAccessFaucetByPriority(faucetType string, available int) bool {
	switch faucetType {
	case FaucetPassive:
		return true
	case FaucetDaily:
		return available >= 10
	case FaucetActivity:
		return available >= 50
	default:
		return false
	}
}

func TryDistributeCoinsWithPriority(faucetType string, amount int) bool {
	available := economy.AvailableCoins()
	if !CanAccessFaucetByPriority(faucetType, available) {
		return false
	}

	return economy.TryDistributeCoins(amount)
}

func CanClaimFaucet(
	db *sql.DB,
	playerID string,
	faucetKey string,
	cooldown time.Duration,
) (bool, time.Duration, error) {
	var lastClaim time.Time

	err := db.QueryRow(`
		SELECT last_claim_at
		FROM player_faucet_claims
		WHERE player_id = $1 AND faucet_key = $2
	`, playerID, faucetKey).Scan(&lastClaim)

	if err == sql.ErrNoRows {
		return true, 0, nil
	}
	if err != nil {
		return false, 0, err
	}

	now := time.Now().UTC()
	next := lastClaim.Add(cooldown)
	if !now.Before(next) {
		return true, 0, nil
	}

	return false, next.Sub(now), nil
}

func RecordFaucetClaim(db *sql.DB, playerID string, faucetKey string) error {
	_, err := db.Exec(`
		INSERT INTO player_faucet_claims (
			player_id,
			faucet_key,
			last_claim_at,
			claim_count
		)
		VALUES ($1, $2, NOW(), 1)
		ON CONFLICT (player_id, faucet_key)
		DO UPDATE SET
			last_claim_at = NOW(),
			claim_count = player_faucet_claims.claim_count + 1
	`, playerID, faucetKey)

	return err
}

type FaucetScaling struct {
	RewardMultiplier   float64
	CooldownMultiplier float64
}

func currentFaucetScaling(now time.Time) FaucetScaling {
	progress := seasonProgress(now)
	reward := 1.35 - (0.65 * progress)
	if reward < 0.7 {
		reward = 0.7
	} else if reward > 1.5 {
		reward = 1.5
	}

	cooldown := 0.7 + (0.9 * progress)
	if cooldown < 0.6 {
		cooldown = 0.6
	} else if cooldown > 1.8 {
		cooldown = 1.8
	}

	return FaucetScaling{
		RewardMultiplier:   reward,
		CooldownMultiplier: cooldown,
	}
}

func applyFaucetRewardScaling(reward int, multiplier float64) int {
	if reward <= 0 {
		return reward
	}
	adjusted := int(float64(reward)*multiplier + 0.9999)
	if adjusted < 1 {
		return 1
	}
	return adjusted
}

func applyFaucetCooldownScaling(cooldown time.Duration, multiplier float64) time.Duration {
	if cooldown <= 0 {
		return cooldown
	}
	adjusted := time.Duration(float64(cooldown) * multiplier)
	if adjusted < time.Second {
		return time.Second
	}
	return adjusted
}
