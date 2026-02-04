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
	var activeCoins int64
	var activePlayers int
	// 24h window aligns with daily cadence and market-pressure lookback.
	const activeEconomyWindow = 24 * time.Hour
	activeSince := time.Now().UTC().Add(-activeEconomyWindow)
	if err := db.QueryRow(`
		SELECT
			COALESCE(SUM(coins), 0) AS total_coins,
			COALESCE(SUM(CASE WHEN last_active_at >= $1 THEN coins ELSE 0 END), 0) AS active_coins,
			COALESCE(COUNT(DISTINCT CASE WHEN last_active_at >= $1 THEN player_id END), 0) AS active_players
		FROM players
	`, activeSince).Scan(&total, &activeCoins, &activePlayers); err != nil {
		log.Println("coins-in-wallets query failed:", err)
		return
	}
	economy.SetCirculationStats(total, activeCoins, activePlayers)
}

func startTickLoop(db *sql.DB) {
	ticker := time.NewTicker(emissionTickInterval)
	startTime := time.Now().UTC()
	setNextEmissionTick(startTime.Add(emissionTickInterval))
	updateTickHeartbeat(db, startTime)
	refreshCoinsInWallets(db)

	go func() {
		tickCount := 0
		for t := range ticker.C {
			now := t.UTC()
			setNextEmissionTick(now.Add(emissionTickInterval))
			if !claimTick(db, now) {
				continue
			}
			log.Println("Tick:", now)

			if isSeasonEnded(now) {
				finalized, err := FinalizeSeason(db, currentSeasonID())
				if err != nil {
					log.Println("Season finalization failed:", err)
				} else if finalized {
					log.Println("Season finalized:", currentSeasonID())
					emitNotification(db, NotificationInput{
						RecipientRole: NotificationRolePlayer,
						Category:      NotificationCategorySystem,
						Type:          "season_ended",
						Priority:      NotificationPriorityHigh,
						Message:       "Season has ended. Final results are available.",
						Payload: map[string]interface{}{
							"seasonId": currentSeasonID(),
						},
						DedupKey:    "season_end:" + currentSeasonID(),
						DedupWindow: 6 * time.Hour,
					})
					emitNotification(db, NotificationInput{
						RecipientRole: NotificationRoleAdmin,
						Category:      NotificationCategorySystem,
						Type:          "season_ended",
						Priority:      NotificationPriorityHigh,
						Message:       "Season finalized: " + currentSeasonID(),
						Payload: map[string]interface{}{
							"seasonId": currentSeasonID(),
						},
						DedupKey:    "season_end_admin:" + currentSeasonID(),
						DedupWindow: 6 * time.Hour,
					})
				}
				continue
			}

			refreshCoinsInWallets(db)

			// Emission: release coins evenly over the day using dynamic season pressure
			activeCoins := economy.ActiveCoinsInCirculation()
			remaining := seasonSecondsRemaining(now)
			dailyTarget := economy.EffectiveDailyEmissionTarget(remaining, activeCoins)
			baseTarget := economy.DailyEmissionTarget()
			if baseTarget > 0 {
				ratio := float64(dailyTarget) / float64(baseTarget)
				if ratio <= 0.7 {
					priority := NotificationPriorityHigh
					if ratio <= 0.5 {
						priority = NotificationPriorityCritical
					}
					emitNotification(db, NotificationInput{
						RecipientRole: NotificationRoleAdmin,
						Category:      NotificationCategoryEconomy,
						Type:          "emission_throttle",
						Priority:      priority,
						Message:       "Daily emission target throttled below baseline.",
						Payload: map[string]interface{}{
							"effectiveTarget": dailyTarget,
							"baseTarget":      baseTarget,
							"ratio":           ratio,
						},
						DedupKey:    "emission_throttle",
						DedupWindow: 45 * time.Minute,
					})
				}
			}

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

			if featureFlags.Telemetry {
				snapshot := economy.InvariantSnapshot()
				emitServerTelemetry(db, nil, "", "emission_tick", map[string]interface{}{
					"seasonId":         currentSeasonID(),
					"emitted":          emitNow,
					"dailyTarget":      dailyTarget,
					"baseTarget":       baseTarget,
					"remainingSeconds": remaining,
					"globalCoinPool":   snapshot.GlobalCoinPool,
					"coinsDistributed": snapshot.CoinsDistributed,
					"availableCoins":   snapshot.AvailableCoins,
				})
			}

			updateMarketPressure(db, now)
			UpdateAbuseMonitoring(db, now)
			checkEconomyInvariants(db, "tick")

			tickCount++
			if tickCount%5 == 0 {
				economy.persist(currentSeasonID(), db)
			}
		}
	}()
}
