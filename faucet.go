package main

import (
	"crypto/rand"
	"database/sql"
	"math/big"
	"time"
)

const (
	FaucetPassive  = "passive"
	FaucetDaily    = "daily"
	FaucetActivity = "activity"
	FaucetRisk     = "risk"
)

func CanAccessFaucetByPriority(faucetType string, available int) bool {
	switch faucetType {
	case FaucetPassive:
		return true
	case FaucetDaily:
		return available >= 10
	case FaucetActivity:
		return available >= 50
	case FaucetRisk:
		return available >= 100
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

func rollWin50() (bool, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(2))
	if err != nil {
		return false, err
	}

	return n.Int64() == 1, nil
}
