package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"
)

const emissionTickInterval = 60 * time.Second
const seasonControlsCacheTTL = 60 * time.Second

var (
	emissionTickMu   sync.RWMutex
	nextEmissionTick time.Time

	seasonControlsMu       sync.RWMutex
	cachedSeasonControls   map[string]interface{}
	lastSeasonControlsLoad time.Time
)

func init() {
	cachedSeasonControls = make(map[string]interface{})
}

func setNextEmissionTick(t time.Time) {
	emissionTickMu.Lock()
	nextEmissionTick = t
	emissionTickMu.Unlock()
}

func loadSeasonControls(db *sql.DB, seasonID string, now time.Time) map[string]interface{} {
	seasonControlsMu.Lock()
	defer seasonControlsMu.Unlock()

	if time.Since(lastSeasonControlsLoad) < seasonControlsCacheTTL && len(cachedSeasonControls) > 0 {
		return cachedSeasonControls
	}

	seasonUUID := deriveUUIDFromString("season", seasonID)
	cachedSeasonControls = make(map[string]interface{})

	rows, err := db.Query(`
		SELECT control_name, value, expires_at
		FROM season_controls
		WHERE season_id = $1
	`, seasonUUID)
	if err != nil {
		log.Println("load season controls: query failed:", err)
		return cachedSeasonControls
	}
	defer rows.Close()

	for rows.Next() {
		var controlName string
		var value []byte
		var expiresAt sql.NullTime
		if err := rows.Scan(&controlName, &value, &expiresAt); err != nil {
			continue
		}

		if expiresAt.Valid && expiresAt.Time.Before(now) {
			continue
		}

		var decoded interface{}
		if err := json.Unmarshal(value, &decoded); err != nil {
			continue
		}
		cachedSeasonControls[controlName] = decoded
	}

	lastSeasonControlsLoad = now
	return cachedSeasonControls
}

func isSeasonFrozen(controls map[string]interface{}) bool {
	val, exists := controls["SEASON_FREEZE"]
	if !exists {
		return false
	}
	frozen, ok := val.(bool)
	return ok && frozen
}

func getEmissionMultiplier(controls map[string]interface{}) float64 {
	val, exists := controls["EMISSION_MULTIPLIER"]
	if !exists {
		return 1.0
	}
	mult, ok := val.(float64)
	if !ok {
		return 1.0
	}
	if mult < 0.5 {
		mult = 0.5
	}
	if mult > 1.5 {
		mult = 1.5
	}
	return mult
}

func getMarketPressureClamp(controls map[string]interface{}) float64 {
	val, exists := controls["MARKET_PRESSURE_RATE_CLAMP"]
	if !exists {
		return 1.0
	}
	clamp, ok := val.(float64)
	if !ok {
		return 1.0
	}
	if clamp < 0.5 {
		clamp = 0.5
	}
	if clamp > 1.0 {
		clamp = 1.0
	}
	return clamp
}

func updateMarketPressureWithClamp(db *sql.DB, now time.Time, clampFactor float64) {
	seasonID := currentSeasonID()
	var last24h int
	var last7d int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM star_purchase_log
		WHERE season_id = $1 AND created_at >= $2
	`, seasonID, now.Add(-24*time.Hour)).Scan(&last24h); err != nil {
		log.Println("market pressure: last24h query failed:", err)
		return
	}
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM star_purchase_log
		WHERE season_id = $1 AND created_at >= $2
	`, seasonID, now.Add(-7*24*time.Hour)).Scan(&last7d); err != nil {
		log.Println("market pressure: last7d query failed:", err)
		return
	}

	longTermDaily := float64(last7d) / 7.0
	if longTermDaily < 1 {
		longTermDaily = 1
	}
	ratio := float64(last24h) / longTermDaily

	desired := 1.0
	if ratio >= 1 {
		desired = 1 + math.Min(0.8, 0.25*(ratio-1))
	} else {
		desired = 1 - math.Min(0.3, 0.15*(1-ratio))
	}

	maxDeltaPerHour := 0.02 * clampFactor
	maxDelta := maxDeltaPerHour / 60
	current := economy.MarketPressure()
	updated := economy.UpdateMarketPressure(desired, maxDelta)
	if featureFlags.Telemetry {
		emitServerTelemetry(db, nil, "", "market_pressure_tick", map[string]interface{}{
			"seasonId":        seasonID,
			"last24h":         last24h,
			"last7d":          last7d,
			"ratio":           ratio,
			"desired":         desired,
			"currentPressure": current,
			"updatedPressure": updated,
			"maxDeltaPerHour": maxDeltaPerHour,
			"maxDeltaPerTick": maxDelta,
			"pressureClamp":   clampFactor,
		})
	}
	if current < 1.5 && updated >= 1.5 {
		priority := NotificationPriorityHigh
		if updated >= 1.7 {
			priority = NotificationPriorityCritical
		}
		emitNotification(db, NotificationInput{
			RecipientRole: NotificationRoleAdmin,
			Category:      NotificationCategoryMarket,
			Type:          "market_pressure_spike",
			Priority:      priority,
			Message:       "Market pressure spike detected.",
			Payload: map[string]interface{}{
				"last24h":        last24h,
				"last7d":         last7d,
				"ratio":          ratio,
				"desired":        desired,
				"marketPressure": updated,
				"pressureClamp":  clampFactor,
			},
			DedupKey:    "market_pressure_spike",
			DedupWindow: 60 * time.Minute,
		})
	}
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
	loadSeasonControls(db, currentSeasonID(), startTime)

	go func() {
		tickCount := 0
		for t := range ticker.C {
			now := t.UTC()
			setNextEmissionTick(now.Add(emissionTickInterval))
			if !claimTick(db, now) {
				continue
			}
			log.Println("Tick:", now)

			seasonControls := loadSeasonControls(db, currentSeasonID(), now)

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

			if isSeasonFrozen(seasonControls) {
				log.Println("Tick aborted: season is frozen")
				continue
			}

			refreshCoinsInWallets(db)

			activeCoins := economy.ActiveCoinsInCirculation()
			remaining := seasonSecondsRemaining(now)
			dailyTarget := economy.EffectiveDailyEmissionTarget(remaining, activeCoins)

			emissionMultiplier := getEmissionMultiplier(seasonControls)
			if emissionMultiplier != 1.0 {
				dailyTarget = int(float64(dailyTarget) * emissionMultiplier)
				log.Println("Emission multiplier applied:", emissionMultiplier, "-> target:", dailyTarget)
			}

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

			pressureClamp := getMarketPressureClamp(seasonControls)
			updateMarketPressureWithClamp(db, now, pressureClamp)
			UpdateAbuseMonitoring(db, now)
			checkEconomyInvariants(db, "tick")

			tickCount++
			if tickCount%5 == 0 {
				economy.persist(currentSeasonID(), db)
			}
		}
	}()
}
