package main

import (
	"database/sql"
)

func FinalizeSeason(db *sql.DB, seasonID string) (bool, error) {
	coins, stars, distributed := economy.Snapshot()

	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		INSERT INTO season_end_snapshots (
			season_id,
			ended_at,
			coins_in_circulation,
			stars_purchased,
			coins_distributed
		)
		VALUES ($1, NOW(), $2, $3, $4)
		ON CONFLICT (season_id) DO NOTHING
	`, seasonID, coins, stars, distributed)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if rows == 0 {
		return false, tx.Commit()
	}

	_, err = tx.Exec(`
		INSERT INTO season_final_rankings (
			season_id,
			player_id,
			stars,
			coins,
			captured_at
		)
		SELECT $1, player_id, stars, coins, NOW()
		FROM players
		ON CONFLICT (season_id, player_id) DO NOTHING
	`, seasonID)
	if err != nil {
		return false, err
	}

	return true, tx.Commit()
}
