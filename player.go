package main

import (
	"database/sql"
	"log"
	"math"
	"strings"
	"time"
)

type Player struct {
	PlayerID        string
	Coins           int64
	Stars           int64
	LastCoinGrantAt time.Time
	LastActiveAt    time.Time
}

const (
	ipDampeningPriceMultiplier  = 1.5
	ipDampeningDelay            = 10 * time.Minute
	ipDampeningRewardMultiplier = 0.7
	ipDampeningMaxAccounts      = 1
)

const (
	trustStatusNormal    = "normal"
	trustStatusThrottled = "throttled"
	trustStatusFlagged   = "flagged"
)

func trustStatusDelayMultiplier(status string) float64 {
	switch status {
	case trustStatusThrottled:
		return 1.5
	case trustStatusFlagged:
		return 2.0
	default:
		return 1.0
	}
}

func trustStatusPriceMultiplier(status string) float64 {
	switch status {
	case trustStatusThrottled:
		return 1.25
	case trustStatusFlagged:
		return 1.5
	default:
		return 1.0
	}
}

func trustStatusRewardMultiplier(status string) float64 {
	switch status {
	case trustStatusThrottled:
		return 0.9
	case trustStatusFlagged:
		return 0.8
	default:
		return 1.0
	}
}

func accountTrustStatusForPlayer(db *sql.DB, playerID string) (string, error) {
	accountID, err := accountIDForPlayer(db, playerID)
	if err != nil {
		return trustStatusNormal, err
	}
	var status string
	err = db.QueryRow(`
		SELECT COALESCE(trust_status, 'normal')
		FROM accounts
		WHERE account_id = $1
	`, accountID).Scan(&status)
	if err == sql.ErrNoRows {
		return trustStatusNormal, nil
	}
	if err != nil {
		return trustStatusNormal, err
	}
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case trustStatusNormal, trustStatusThrottled, trustStatusFlagged:
		return status, nil
	default:
		return trustStatusNormal, nil
	}
}

func LoadOrCreatePlayer(
	db *sql.DB,
	playerID string,
) (*Player, error) {

	var p Player

	err := db.QueryRow(`
		SELECT player_id, coins, stars, last_coin_grant_at, last_active_at
		FROM players
		WHERE player_id = $1
	`, playerID).Scan(&p.PlayerID, &p.Coins, &p.Stars, &p.LastCoinGrantAt, &p.LastActiveAt)

	if err == nil {
		now := time.Now().UTC()
		if isSeasonEnded(now) {
			return &p, nil
		}
		return &p, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create new player (NO starter coins yet)
	_, err = db.Exec(`
		INSERT INTO players (
			player_id,
			coins,
			stars,
			created_at,
			last_active_at,
			last_coin_grant_at
		)
		VALUES ($1, 0, 0, NOW(), NOW(), NOW())
	`, playerID)
	if err != nil {
		return nil, err
	}

	return &Player{
		PlayerID: playerID,
		Coins:    0,
		Stars:    0,
	}, nil
}

func LoadPlayer(db *sql.DB, playerID string) (*Player, error) {
	var p Player

	err := db.QueryRow(`
		SELECT player_id, coins, stars, last_coin_grant_at
		FROM players
		WHERE player_id = $1
	`, playerID).Scan(&p.PlayerID, &p.Coins, &p.Stars, &p.LastCoinGrantAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func RecordPlayerIP(db *sql.DB, playerID string, ip string) (bool, error) {
	if ip == "" {
		return false, nil
	}

	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM player_ip_associations
			WHERE player_id = $1 AND ip = $2
		)
	`, playerID, ip).Scan(&exists)
	if err != nil {
		return false, err
	}

	_, err = db.Exec(`
		INSERT INTO player_ip_associations (
			player_id,
			ip,
			first_seen,
			last_seen
		)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (player_id, ip)
		DO UPDATE SET
			last_seen = NOW()
	`, playerID, ip)

	if err != nil {
		return false, err
	}

	return !exists, nil
}

func ApplyIPDampeningDelay(db *sql.DB, playerID string, ip string) error {
	if !featureFlags.IPThrottling {
		return nil
	}
	if ip == "" {
		return nil
	}

	count, err := countPlayersForIP(db, ip)
	if err != nil {
		return err
	}

	if count <= ipDampeningMaxAccounts {
		return nil
	}

	trustStatus, err := accountTrustStatusForPlayer(db, playerID)
	if err != nil {
		log.Printf("ip_dampening: trust_status lookup failed (player_id=%s, ip=%s): %v", playerID, ip, err)
		trustStatus = trustStatusNormal
	}

	log.Printf("ip_dampening: delay applied (player_id=%s, ip=%s, count=%d, trust_status=%s)", playerID, ip, count, trustStatus)

	delaySeconds := int(ipDampeningDelay.Seconds() * trustStatusDelayMultiplier(trustStatus))
	_, err = db.Exec(`
		UPDATE players
		SET last_coin_grant_at = NOW() + ($2 * INTERVAL '1 second')
		WHERE player_id = $1
	`, playerID, delaySeconds)

	return err
}

func IsPlayerThrottledByIP(db *sql.DB, playerID string) (bool, error) {
	if !featureFlags.IPThrottling {
		return false, nil
	}
	ip, err := latestIPForPlayer(db, playerID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if ip == "" {
		return false, nil
	}

	count, err := countPlayersForIP(db, ip)
	if err != nil {
		return false, err
	}
	if count <= ipDampeningMaxAccounts {
		return false, nil
	}

	return true, nil
}

func ComputeDampenedStarPrice(db *sql.DB, playerID string, basePrice int) (int, error) {
	if !featureFlags.IPThrottling {
		return basePrice, nil
	}
	throttled, err := IsPlayerThrottledByIP(db, playerID)
	if err != nil {
		return basePrice, err
	}
	if !throttled {
		return basePrice, nil
	}

	trustStatus, err := accountTrustStatusForPlayer(db, playerID)
	if err != nil {
		trustStatus = trustStatusNormal
	}
	multiplier := ipDampeningPriceMultiplier * trustStatusPriceMultiplier(trustStatus)
	// Round UP (ceil) for prices to ensure conservative charging of anti-abuse multipliers
	return int(math.Ceil(float64(basePrice) * multiplier)), nil
}

func ApplyIPDampeningReward(db *sql.DB, playerID string, reward int) (int, error) {
	if !featureFlags.IPThrottling {
		return reward, nil
	}
	if reward <= 0 {
		return reward, nil
	}
	throttled, err := IsPlayerThrottledByIP(db, playerID)
	if err != nil {
		return reward, err
	}
	if !throttled {
		return reward, nil
	}
	trustStatus, err := accountTrustStatusForPlayer(db, playerID)
	if err != nil {
		trustStatus = trustStatusNormal
	}
	multiplier := ipDampeningRewardMultiplier * trustStatusRewardMultiplier(trustStatus)
	// Round DOWN (floor) for rewards to ensure conservative earning when throttled
	adjusted := int(math.Floor(float64(reward) * multiplier))
	if adjusted < 1 {
		adjusted = 1
	}
	return adjusted, nil
}

func latestIPForPlayer(db *sql.DB, playerID string) (string, error) {
	var ip string

	err := db.QueryRow(`
		SELECT ip
		FROM player_ip_associations
		WHERE player_id = $1
		ORDER BY last_seen DESC
		LIMIT 1
	`, playerID).Scan(&ip)

	if err != nil {
		return "", err
	}

	return ip, nil
}

func countPlayersForIP(db *sql.DB, ip string) (int, error) {
	var count int
	if ip == "" {
		return 0, nil
	}

	err := db.QueryRow(`
		SELECT COUNT(DISTINCT player_id)
		FROM player_ip_associations
		WHERE ip = $1
	`, ip).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func UpdatePlayerBalances(
	db *sql.DB,
	playerID string,
	coins int64,
	stars int64,
) error {

	_, err := db.Exec(`
		UPDATE players
		SET coins = $2,
			stars = $3,
			last_active_at = NOW()
		WHERE player_id = $1
	`, playerID, coins, stars)

	return err
}
