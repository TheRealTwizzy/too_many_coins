package main

import (
	"database/sql"
	"log"
	"sync"
)

type EconomyState struct {
	mu                  sync.Mutex
	globalCoinPool      int
	dailyEmissionTarget int
	emissionRemainder   float64
}

var economy = &EconomyState{
	globalCoinPool:      0,
	dailyEmissionTarget: 1000,
	emissionRemainder:   0,
}

func (e *EconomyState) emitCoins(amount int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.globalCoinPool += amount
	log.Println("Economy: emitted coins,", amount, "pool now", e.globalCoinPool)
}

func (e *EconomyState) persist(seasonID string, db *sql.DB) {
	e.mu.Lock()
	defer e.mu.Unlock()

	_, err := db.Exec(`
		INSERT INTO season_economy (season_id, global_coin_pool, emission_remainder, last_updated)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (season_id)
		DO UPDATE SET
			global_coin_pool = EXCLUDED.global_coin_pool,
			emission_remainder = EXCLUDED.emission_remainder,
			last_updated = NOW()
	`,
		seasonID,
		e.globalCoinPool,
		e.emissionRemainder,
	)

	if err != nil {
		log.Println("Economy persist error:", err)
	}
}

func (e *EconomyState) load(seasonID string, db *sql.DB) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	row := db.QueryRow(`
		SELECT global_coin_pool, emission_remainder
		FROM season_economy
		WHERE season_id = $1
	`, seasonID)

	var pool int64
	var remainder float64

	err := row.Scan(&pool, &remainder)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("Economy: no existing state, starting fresh")
			return nil
		}
		return err
	}

	e.globalCoinPool = int(pool)
	e.emissionRemainder = remainder

	log.Println("Economy: loaded state, pool", e.globalCoinPool)
	return nil
}
