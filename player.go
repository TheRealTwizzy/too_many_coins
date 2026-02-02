package main

import (
	"database/sql"
)

type Player struct {
	PlayerID string
	Coins    int64
	Stars    int64
}

func LoadOrCreatePlayer(
	db *sql.DB,
	playerID string,
) (*Player, error) {

	var p Player

	err := db.QueryRow(`
		SELECT player_id, coins, stars
		FROM players
		WHERE player_id = $1
	`, playerID).Scan(&p.PlayerID, &p.Coins, &p.Stars)

	if err == nil {
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
			last_active_at
		)
		VALUES ($1, 0, 0, NOW(), NOW())
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
