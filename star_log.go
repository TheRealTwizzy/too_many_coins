package main

import (
	"database/sql"
)

type StarPurchaseLogEntry struct {
	ID           int64
	AccountID    string
	PlayerID     string
	SeasonID     string
	PurchaseType string
	Variant      string
	PricePaid    int64
	CoinsBefore  int64
	CoinsAfter   int64
	StarsBefore  int64
	StarsAfter   int64
	CreatedAt    string
}

func logStarPurchase(db *sql.DB, accountID string, playerID string, seasonID string, purchaseType string, variant string, pricePaid int, coinsBefore int64, coinsAfter int64, starsBefore int64, starsAfter int64) {
	_, _ = db.Exec(`
		INSERT INTO star_purchase_log (
			account_id,
			player_id,
			season_id,
			purchase_type,
			variant,
			price_paid,
			coins_before,
			coins_after,
			stars_before,
			stars_after,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
	`, accountID, playerID, seasonID, purchaseType, variant, pricePaid, coinsBefore, coinsAfter, starsBefore, starsAfter)
}

func logStarPurchaseTx(tx *sql.Tx, accountID string, playerID string, seasonID string, purchaseType string, variant string, pricePaid int, coinsBefore int64, coinsAfter int64, starsBefore int64, starsAfter int64) error {
	_, err := tx.Exec(`
		INSERT INTO star_purchase_log (
			account_id,
			player_id,
			season_id,
			purchase_type,
			variant,
			price_paid,
			coins_before,
			coins_after,
			stars_before,
			stars_after,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
	`, accountID, playerID, seasonID, purchaseType, variant, pricePaid, coinsBefore, coinsAfter, starsBefore, starsAfter)
	return err
}
