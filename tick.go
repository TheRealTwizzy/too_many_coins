package main

import (
	"database/sql"
	"log"
	"time"
)

func startTickLoop(db *sql.DB) {
	ticker := time.NewTicker(60 * time.Second)

	go func() {
		tickCount := 0
		for t := range ticker.C {
			now := t.UTC()
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

			// Simple emission: release coins evenly over the day
			economy.mu.Lock()

			coinsPerTick := float64(economy.dailyEmissionTarget) / (24 * 60)
			economy.emissionRemainder += coinsPerTick

			emitNow := int(economy.emissionRemainder)
			if emitNow > 0 {
				economy.emissionRemainder -= float64(emitNow)
				economy.globalCoinPool += emitNow
				log.Println("Economy: emitted coins,", emitNow, "pool now", economy.globalCoinPool)
			}

			economy.mu.Unlock()

			tickCount++
			if tickCount%5 == 0 {
				economy.persist(currentSeasonID(), db)
			}
		}
	}()
}
