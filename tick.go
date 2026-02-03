package main

import (
	"database/sql"
	"log"
	"sync"
	"time"
)

const emissionTickInterval = 60 * time.Second

var (
	emissionTickMu   sync.RWMutex
	nextEmissionTick time.Time
)

func setNextEmissionTick(t time.Time) {
	emissionTickMu.Lock()
	nextEmissionTick = t
	emissionTickMu.Unlock()
}

func nextEmissionSeconds(now time.Time) int64 {
	emissionTickMu.RLock()
	next := nextEmissionTick
	emissionTickMu.RUnlock()
	if next.IsZero() {
		return int64(emissionTickInterval.Seconds())
	}
	remaining := next.Sub(now)
	if remaining < 0 {
		return 0
	}
	return int64(remaining.Seconds())
}

func refreshCoinsInWallets(db *sql.DB) {
	var total int64
	if err := db.QueryRow(`
		SELECT COALESCE(SUM(coins), 0)
		FROM players
	`).Scan(&total); err != nil {
		log.Println("coins-in-wallets query failed:", err)
		return
	}
	economy.SetCoinsInWallets(total)
}

func startTickLoop(db *sql.DB) {
	ticker := time.NewTicker(emissionTickInterval)
	setNextEmissionTick(time.Now().UTC().Add(emissionTickInterval))
	refreshCoinsInWallets(db)

	go func() {
		tickCount := 0
		for t := range ticker.C {
			now := t.UTC()
			setNextEmissionTick(now.Add(emissionTickInterval))
			log.Println("Tick:", now)

			if isSeasonEnded(now) {
				finalized, err := FinalizeSeason(db, currentSeasonID())
				if err != nil {
					log.Println("Season finalization failed:", err)
				} else if finalized {
					log.Println("Season finalized:", currentSeasonID())
				}
				continue
			}

			refreshCoinsInWallets(db)

			// Emission: release coins evenly over the day using dynamic season pressure
			coinsInCirculation := economy.CoinsInCirculation()
			remaining := seasonSecondsRemaining(now)
			dailyTarget := economy.EffectiveDailyEmissionTarget(remaining, coinsInCirculation)

			economy.mu.Lock()
			coinsPerTick := float64(dailyTarget) / (24 * 60)
			economy.emissionRemainder += coinsPerTick

			emitNow := int(economy.emissionRemainder)
			if emitNow > 0 {
				economy.emissionRemainder -= float64(emitNow)
				economy.globalCoinPool += emitNow
				log.Println("Economy: emitted coins,", emitNow, "pool now", economy.globalCoinPool)
			}

			economy.mu.Unlock()

			updateMarketPressure(db, now)

			tickCount++
			if tickCount%5 == 0 {
				economy.persist(currentSeasonID(), db)
			}
		}
	}()
}
