package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"
)

func emitServerTelemetry(db *sql.DB, accountID *string, playerID string, eventType string, payload map[string]interface{}) {
	if db == nil || eventType == "" {
		return
	}
	if !featureFlags.Telemetry {
		log.Println("telemetry disabled:", eventType, payload)
		return
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		log.Println("telemetry marshal failed:", err)
		return
	}
	accountValue := ""
	if accountID != nil {
		accountValue = *accountID
	}
	_, err = db.Exec(`
		INSERT INTO player_telemetry (account_id, player_id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, nullableString(accountValue), playerID, eventType, encoded)
	if err != nil {
		log.Println("telemetry insert failed:", err)
	}
}

func emitServerTelemetryWithCooldown(db *sql.DB, accountID *string, playerID string, eventType string, payload map[string]interface{}, cooldown time.Duration) {
	if db == nil || eventType == "" {
		return
	}
	if cooldown > 0 {
		var last time.Time
		err := db.QueryRow(`
			SELECT created_at
			FROM player_telemetry
			WHERE player_id = $1 AND event_type = $2
			ORDER BY created_at DESC
			LIMIT 1
		`, playerID, eventType).Scan(&last)
		if err == nil && time.Since(last) < cooldown {
			return
		}
	}
	emitServerTelemetry(db, accountID, playerID, eventType, payload)
}

func logFaucetDenied(db *sql.DB, accountID *string, playerID string, faucetType string, reason string, payload map[string]interface{}) {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payload["faucet"] = faucetType
	payload["reason"] = reason
	emitServerTelemetryWithCooldown(db, accountID, playerID, "faucet_denied", payload, 2*time.Minute)
}

func checkEconomyInvariants(db *sql.DB, context string) {
	snapshot := economy.InvariantSnapshot()
	violations := []string{}
	if snapshot.GlobalCoinPool < 0 {
		violations = append(violations, "global_coin_pool_negative")
	}
	if snapshot.AvailableCoins < 0 {
		violations = append(violations, "available_coins_negative")
	}
	if snapshot.CoinsDistributed > snapshot.GlobalCoinPool {
		violations = append(violations, "coins_distributed_exceeds_pool")
	}
	if snapshot.MarketPressure < 0.6 || snapshot.MarketPressure > 1.8 {
		violations = append(violations, "market_pressure_out_of_bounds")
	}
	if len(violations) == 0 {
		return
	}

	payload := map[string]interface{}{
		"context":          context,
		"violations":       violations,
		"globalCoinPool":   snapshot.GlobalCoinPool,
		"coinsDistributed": snapshot.CoinsDistributed,
		"availableCoins":   snapshot.AvailableCoins,
		"marketPressure":   snapshot.MarketPressure,
	}
	log.Println("ECONOMY INVARIANT VIOLATION:", payload)
	emitServerTelemetryWithCooldown(db, nil, "", "economy_invariant_violation", payload, 5*time.Minute)
	emitNotification(db, NotificationInput{
		RecipientRole: NotificationRoleAdmin,
		Category:      NotificationCategoryEconomy,
		Type:          "economy_invariant_violation",
		Priority:      NotificationPriorityCritical,
		Message:       "Economy invariant violation detected.",
		Payload:       payload,
		DedupKey:      "economy_invariant_violation",
		DedupWindow:   10 * time.Minute,
	})
}

func verifyDailyPlayability(db *sql.DB, playerID string, accountID *string) {
	now := time.Now().UTC()
	if isSeasonEnded(now) {
		return
	}
	if playerID == "" {
		return
	}

	var coins int64
	var lastActive time.Time
	if err := db.QueryRow(`
		SELECT coins, last_active_at
		FROM players
		WHERE player_id = $1
	`, playerID).Scan(&coins, &lastActive); err != nil {
		emitServerTelemetryWithCooldown(db, accountID, playerID, "playability_check_error", map[string]interface{}{
			"reason": "player_lookup_failed",
			"error":  err.Error(),
		}, 10*time.Minute)
		return
	}

	remainingCap, err := RemainingDailyCap(db, playerID, now)
	if err != nil {
		emitServerTelemetryWithCooldown(db, accountID, playerID, "playability_check_error", map[string]interface{}{
			"reason": "daily_cap_lookup_failed",
			"error":  err.Error(),
		}, 10*time.Minute)
		return
	}

	coinsInCirculation := economy.CoinsInCirculation()
	secondsRemaining := seasonSecondsRemaining(now)
	currentPrice := ComputeSeasonAuthorityStarPrice(coinsInCirculation, secondsRemaining)
	canBuyStar := featureFlags.SinksEnabled && coins >= int64(currentPrice)

	canClaimDaily := false
	canClaimActivity := false
	if featureFlags.FaucetsEnabled && remainingCap > 0 {
		params := economy.Calibration()
		activityWindow := ActiveActivityWindow()
		isActive := now.Sub(lastActive) <= activityWindow

		dailyReward := params.DailyLoginReward
		dailyCooldown := time.Duration(params.DailyLoginCooldownHours) * time.Hour
		enforcement := abuseEffectiveEnforcement(db, playerID, bulkStarMaxQty())
		dailyReward = abuseAdjustedReward(dailyReward, enforcement.EarnMultiplier)
		dailyCooldown += abuseCooldownJitter(dailyCooldown, enforcement.CooldownJitterFactor)
		scaling := currentFaucetScaling(now)
		dailyReward = applyFaucetRewardScaling(dailyReward, scaling.RewardMultiplier)
		dailyCooldown = applyFaucetCooldownScaling(dailyCooldown, scaling.CooldownMultiplier)
		if reward, err := ApplyIPDampeningReward(db, playerID, dailyReward); err == nil {
			dailyReward = reward
		}
		if dailyReward > remainingCap {
			dailyReward = remainingCap
		}
		if dailyReward > 0 {
			if canClaim, _, err := CanClaimFaucet(db, playerID, FaucetDaily, dailyCooldown); err == nil && canClaim {
				available := economy.AvailableCoins()
				adjusted := ThrottleFaucetReward(FaucetDaily, dailyReward, available)
				if adjusted > 0 && CanAccessFaucetByPriority(FaucetDaily, available) {
					canClaimDaily = true
				}
			}
		}

		activityReward := params.ActivityReward
		activityCooldown := time.Duration(params.ActivityCooldownSeconds) * time.Second
		if active, _, err := HasActiveBoost(db, playerID, BoostActivity); err == nil && active {
			activityReward += 1
		}
		activityReward = abuseAdjustedReward(activityReward, enforcement.EarnMultiplier)
		activityCooldown += abuseCooldownJitter(activityCooldown, enforcement.CooldownJitterFactor)
		activityReward = applyFaucetRewardScaling(activityReward, scaling.RewardMultiplier)
		activityCooldown = applyFaucetCooldownScaling(activityCooldown, scaling.CooldownMultiplier)
		if reward, err := ApplyIPDampeningReward(db, playerID, activityReward); err == nil {
			activityReward = reward
		}
		if activityReward > remainingCap {
			activityReward = remainingCap
		}
		if activityReward > 0 && isActive {
			if canClaim, _, err := CanClaimFaucet(db, playerID, FaucetActivity, activityCooldown); err == nil && canClaim {
				available := economy.AvailableCoins()
				adjusted := ThrottleFaucetReward(FaucetActivity, activityReward, available)
				if adjusted > 0 && CanAccessFaucetByPriority(FaucetActivity, available) {
					canClaimActivity = true
				}
			}
		}
	}

	if canBuyStar || canClaimDaily || canClaimActivity {
		return
	}

	emitServerTelemetryWithCooldown(db, accountID, playerID, "player_zero_valid_actions_today", map[string]interface{}{
		"coins":             coins,
		"currentStarPrice":  currentPrice,
		"remainingDailyCap": remainingCap,
		"canBuyStar":        canBuyStar,
		"canClaimDaily":     canClaimDaily,
		"canClaimActivity":  canClaimActivity,
		"availableCoins":    economy.AvailableCoins(),
	}, 6*time.Hour)
}
