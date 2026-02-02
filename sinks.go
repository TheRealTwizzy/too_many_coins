package main

import (
	"database/sql"
	"time"
)

const (
	StarVariantEmber = "ember"
	StarVariantVoid  = "void"
	BoostActivity    = "activity"
)

func AddStarVariant(db *sql.DB, playerID string, variant string, count int) error {
	_, err := db.Exec(`
		INSERT INTO player_star_variants (player_id, variant, count)
		VALUES ($1, $2, $3)
		ON CONFLICT (player_id, variant)
		DO UPDATE SET count = player_star_variants.count + EXCLUDED.count
	`, playerID, variant, count)

	return err
}

func SetBoost(db *sql.DB, playerID string, boostType string, duration time.Duration) (time.Time, error) {
	expiresAt := time.Now().UTC().Add(duration)
	_, err := db.Exec(`
		INSERT INTO player_boosts (player_id, boost_type, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (player_id, boost_type)
		DO UPDATE SET expires_at = EXCLUDED.expires_at
	`, playerID, boostType, expiresAt)

	return expiresAt, err
}

func HasActiveBoost(db *sql.DB, playerID string, boostType string) (bool, time.Time, error) {
	var expiresAt time.Time

	err := db.QueryRow(`
		SELECT expires_at
		FROM player_boosts
		WHERE player_id = $1 AND boost_type = $2
	`, playerID, boostType).Scan(&expiresAt)
	if err == sql.ErrNoRows {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}

	if time.Now().UTC().Before(expiresAt) {
		return true, expiresAt, nil
	}

	return false, expiresAt, nil
}
