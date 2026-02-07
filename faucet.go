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
	FaucetUBI      = "ubi"
)

func CanAccessFaucetByPriority(faucetType string, available int) bool {
	return available > 0
}

func ThrottleFaucetReward(faucetType string, amount int, available int) int {
	if amount <= 0 || available <= 0 {
		return 0
	}
	if amount > available {
		return available
	}
	return amount
}

func TryDistributeCoinsWithPriority(faucetType string, amount int) (int, bool) {
	available := economy.AvailableCoins()
	if !CanAccessFaucetByPriority(faucetType, available) {
		return 0, false
	}
	adjusted := ThrottleFaucetReward(faucetType, amount, available)
	if adjusted <= 0 {
		return 0, false
	}
	if !economy.TryDistributeCoins(adjusted) {
		return 0, false
	}
	return adjusted, true
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
	reward := 1.6 - (1.0 * progress)
	if reward < 0.6 {
		reward = 0.6
	} else if reward > 1.6 {
		reward = 1.6
	}

	cooldown := 0.55 + (1.15 * progress)
	if cooldown < 0.5 {
		cooldown = 0.5
	} else if cooldown > 1.7 {
		cooldown = 1.7
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

// DistributeUniversalBasicIncome grants the minimum 0.001 coin (1 unit) to all eligible players.
// UBI is foundation income: always-on, emission-backed, non-negotiable.
// Failures per-player do not block UBI for other players.
func DistributeUniversalBasicIncome(db *sql.DB, now time.Time) (ubiCount int, ubiTotal int, poolExhausted bool) {
	const ubiPerTick = 1 // Represents 0.001 coins in decimal spec

	if db == nil {
		return 0, 0, false
	}

	seasonID := currentSeasonID()
	if seasonID == "" {
		return 0, 0, false
	}

	// Query all players in the current season
	rows, err := db.Query(`
		SELECT player_id
		FROM players
		ORDER BY player_id
	`)
	if err != nil {
		return 0, 0, false
	}
	defer rows.Close()

	granted := 0
	total := 0

	for rows.Next() {
		var playerID string
		if err := rows.Scan(&playerID); err != nil {
			continue
		}

		// Check pool availability before attempting grant
		available := economy.AvailableCoins()
		if available < ubiPerTick {
			poolExhausted = true
			break
		}

		// Grant UBI with no daily cap (foundation income)
		grantedAmount, err := GrantCoinsNoCap(db, playerID, ubiPerTick, now, FaucetUBI, nil)
		if err != nil {
			continue
		}
		if grantedAmount > 0 {
			granted++
			total += grantedAmount
		}
	}

	if featureFlags.Telemetry && (granted > 0 || poolExhausted) {
		emitServerTelemetry(db, nil, "", "ubi_tick", map[string]interface{}{
			"seasonId":       seasonID,
			"ubiPerTick":     ubiPerTick,
			"playersGranted": granted,
			"totalGranted":   total,
			"poolExhausted":  poolExhausted,
			"availableCoins": economy.AvailableCoins(),
		})
	}

	return granted, total, poolExhausted
}
