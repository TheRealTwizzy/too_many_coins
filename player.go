package main

import (
	"database/sql"
	"time"
)

type Player struct {
	PlayerID        string
	Coins           int64
	Stars           int64
	LastCoinGrantAt time.Time
}

const (
	maxPlayersPerIP            = 2
	ipDampeningPriceMultiplier = 1.5
	ipDampeningDelay           = 10 * time.Minute
)

func LoadOrCreatePlayer(
	db *sql.DB,
	playerID string,
) (*Player, error) {

	var p Player

	err := db.QueryRow(`
		SELECT player_id, coins, stars, last_coin_grant_at
		FROM players
		WHERE player_id = $1
	`, playerID).Scan(&p.PlayerID, &p.Coins, &p.Stars, &p.LastCoinGrantAt)

	if err == nil {
		now := time.Now().UTC()
		if isSeasonEnded(now) {
			_, _ = db.Exec(`
				UPDATE players
				SET last_active_at = NOW()
				WHERE player_id = $1
			`, playerID)
			return &p, nil
		}

		elapsed := now.Sub(p.LastCoinGrantAt)
		minutes := int64(elapsed / time.Minute)

		if minutes > 0 {
			available := economy.AvailableCoins()
			grant := int64(minutes)

			if int(grant) > available {
				grant = int64(available)
			}

			if grant > 0 {
				p.Coins += grant
				economy.mu.Lock()
				economy.coinsDistributed += int(grant)
				economy.mu.Unlock()
				p.LastCoinGrantAt = now
			}

			p.LastCoinGrantAt = now

			_, err = db.Exec(`
			UPDATE players
			SET coins = $2,
				last_coin_grant_at = $3,
				last_active_at = NOW()
			WHERE player_id = $1
		`, p.PlayerID, p.Coins, p.LastCoinGrantAt)

			if err != nil {
				return nil, err
			}
		}

		// Update last_active_at
		_, _ = db.Exec(`
			UPDATE players
			SET last_active_at = NOW()
			WHERE player_id = $1
		`, playerID)

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
	if ip == "" {
		return nil
	}

	count, err := countPlayersForIP(db, ip)
	if err != nil {
		return err
	}

	if count <= maxPlayersPerIP {
		return nil
	}

	delaySeconds := int(ipDampeningDelay.Seconds())
	_, err = db.Exec(`
		UPDATE players
		SET last_coin_grant_at = NOW() + ($2 * INTERVAL '1 second')
		WHERE player_id = $1
	`, playerID, delaySeconds)

	return err
}

func IsPlayerAllowedByIP(db *sql.DB, playerID string) (bool, error) {
	ip, err := latestIPForPlayer(db, playerID)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	if ip == "" {
		return true, nil
	}

	count, err := countPlayersForIP(db, ip)
	if err != nil {
		return true, err
	}

	if count <= maxPlayersPerIP {
		return true, nil
	}

	rows, err := db.Query(`
		SELECT player_id
		FROM player_ip_associations
		WHERE ip = $1
		ORDER BY first_seen ASC
		LIMIT $2
	`, ip, maxPlayersPerIP)
	if err != nil {
		return true, err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return true, err
		}
		if id == playerID {
			return true, nil
		}
	}

	if err := rows.Err(); err != nil {
		return true, err
	}

	return false, nil
}

func ComputeDampenedStarPrice(db *sql.DB, playerID string, basePrice int) (int, error) {
	allowed, err := IsPlayerAllowedByIP(db, playerID)
	if err != nil {
		return basePrice, err
	}
	if allowed {
		return basePrice, nil
	}

	return int(float64(basePrice)*ipDampeningPriceMultiplier + 0.9999), nil
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
