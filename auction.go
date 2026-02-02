package main

import (
	"database/sql"
	"errors"
	"time"
)

const (
	AuctionID         = "daily-cosmetic"
	AuctionItemKey    = "ember-crest"
	AuctionMinBid     = 50
	AuctionDuration   = 10 * time.Minute
	AuctionStarReward = StarVariantEmber
)

type AuctionStatus struct {
	AuctionID     string    `json:"auctionId"`
	ItemKey       string    `json:"itemKey"`
	MinBid        int       `json:"minBid"`
	CurrentBid    int       `json:"currentBid"`
	CurrentWinner string    `json:"currentWinner,omitempty"`
	EndsAt        time.Time `json:"endsAt"`
}

var errAuctionNotFound = errors.New("auction_not_found")

func EnsureAuction(db *sql.DB) error {
	_, err := db.Exec(`
		INSERT INTO system_auctions (
			auction_id,
			item_key,
			min_bid,
			current_bid,
			current_winner,
			ends_at,
			settled_at
		)
		VALUES ($1, $2, $3, 0, NULL, NOW() + ($4 * INTERVAL '1 second'), NULL)
		ON CONFLICT (auction_id) DO NOTHING
	`, AuctionID, AuctionItemKey, AuctionMinBid, int(AuctionDuration.Seconds()))

	return err
}

func GetAuctionStatus(db *sql.DB) (*AuctionStatus, error) {
	if err := settleAuctionIfNeeded(db); err != nil {
		return nil, err
	}

	var status AuctionStatus
	err := db.QueryRow(`
		SELECT auction_id, item_key, min_bid, current_bid, COALESCE(current_winner, ''), ends_at
		FROM system_auctions
		WHERE auction_id = $1
	`, AuctionID).Scan(
		&status.AuctionID,
		&status.ItemKey,
		&status.MinBid,
		&status.CurrentBid,
		&status.CurrentWinner,
		&status.EndsAt,
	)
	if err == sql.ErrNoRows {
		return nil, errAuctionNotFound
	}
	if err != nil {
		return nil, err
	}

	return &status, nil
}

func PlaceAuctionBid(db *sql.DB, playerID string, bid int) (*AuctionStatus, error) {
	if err := EnsureAuction(db); err != nil {
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var (
		itemKey       string
		minBid        int
		currentBid    int
		currentWinner sql.NullString
		endsAt        time.Time
		settledAt     sql.NullTime
	)

	err = tx.QueryRow(`
		SELECT item_key, min_bid, current_bid, current_winner, ends_at, settled_at
		FROM system_auctions
		WHERE auction_id = $1
		FOR UPDATE
	`, AuctionID).Scan(&itemKey, &minBid, &currentBid, &currentWinner, &endsAt, &settledAt)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if !endsAt.After(now) {
		if currentWinner.Valid && !settledAt.Valid {
			if err := AddStarVariantTx(tx, currentWinner.String, AuctionStarReward, 1); err != nil {
				return nil, err
			}
			_, err = tx.Exec(`
				UPDATE system_auctions
				SET settled_at = NOW()
				WHERE auction_id = $1
			`, AuctionID)
			if err != nil {
				return nil, err
			}
		}

		currentBid = 0
		currentWinner = sql.NullString{}
		endsAt = now.Add(AuctionDuration)
		_, err = tx.Exec(`
			UPDATE system_auctions
			SET current_bid = 0,
			    current_winner = NULL,
			    ends_at = $2,
			    settled_at = NULL
			WHERE auction_id = $1
		`, AuctionID, endsAt)
		if err != nil {
			return nil, err
		}
	}

	if bid < minBid || bid <= currentBid {
		return nil, errors.New("BID_TOO_LOW")
	}

	var playerCoins int64
	err = tx.QueryRow(`
		SELECT coins
		FROM players
		WHERE player_id = $1
		FOR UPDATE
	`, playerID).Scan(&playerCoins)
	if err != nil {
		return nil, err
	}
	if playerCoins < int64(bid) {
		return nil, errors.New("NOT_ENOUGH_COINS")
	}

	if currentWinner.Valid {
		_, err = tx.Exec(`
			UPDATE players
			SET coins = coins + $2
			WHERE player_id = $1
		`, currentWinner.String, currentBid)
		if err != nil {
			return nil, err
		}
	}

	_, err = tx.Exec(`
		UPDATE players
		SET coins = coins - $2
		WHERE player_id = $1
	`, playerID, bid)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`
		UPDATE system_auctions
		SET current_bid = $2,
		    current_winner = $3
		WHERE auction_id = $1
	`, AuctionID, bid, playerID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`
		INSERT INTO auction_bids (auction_id, player_id, bid, created_at)
		VALUES ($1, $2, $3, NOW())
	`, AuctionID, playerID, bid)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return GetAuctionStatus(db)
}

func settleAuctionIfNeeded(db *sql.DB) error {
	if err := EnsureAuction(db); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		currentWinner sql.NullString
		endsAt        time.Time
		settledAt     sql.NullTime
	)

	err = tx.QueryRow(`
		SELECT current_winner, ends_at, settled_at
		FROM system_auctions
		WHERE auction_id = $1
		FOR UPDATE
	`, AuctionID).Scan(&currentWinner, &endsAt, &settledAt)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if endsAt.After(now) || settledAt.Valid || !currentWinner.Valid {
		return tx.Commit()
	}

	if err := AddStarVariantTx(tx, currentWinner.String, AuctionStarReward, 1); err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE system_auctions
		SET settled_at = NULL,
		    current_bid = 0,
		    current_winner = NULL,
		    ends_at = NOW() + ($2 * INTERVAL '1 second')
		WHERE auction_id = $1
	`, AuctionID, int(AuctionDuration.Seconds()))
	if err != nil {
		return err
	}

	return tx.Commit()
}

func AddStarVariantTx(tx *sql.Tx, playerID string, variant string, count int) error {
	_, err := tx.Exec(`
		INSERT INTO player_star_variants (player_id, variant, count)
		VALUES ($1, $2, $3)
		ON CONFLICT (player_id, variant)
		DO UPDATE SET count = player_star_variants.count + EXCLUDED.count
	`, playerID, variant, count)
	return err
}
