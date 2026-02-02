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
