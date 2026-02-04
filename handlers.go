package main

import (
	"database/sql"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.FileServer(http.FS(publicFS)).ServeHTTP(w, r)
		return
	}

	data, err := fs.ReadFile(publicFS, "index.html")
	if err != nil {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			cwd = "unknown"
		}
		entries, dirErr := fs.ReadDir(publicFS, ".")
		if dirErr != nil {
			log.Printf("serveIndex: failed to read index.html (cwd=%s): %v; readdir error: %v", cwd, err, dirErr)
		} else {
			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			log.Printf("serveIndex: failed to read index.html (cwd=%s): %v; public entries: %s", cwd, err, strings.Join(names, ","))
		}
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

func healthHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "db_unreachable", http.StatusServiceUnavailable)
			return
		}

		var seasonExists bool
		if err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM season_economy
				WHERE season_id = $1
			)
		`, currentSeasonID()).Scan(&seasonExists); err != nil || !seasonExists {
			http.Error(w, "season_missing", http.StatusServiceUnavailable)
			return
		}

		var calibrationExists bool
		if err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM season_calibration
				WHERE season_id = $1
			)
		`, currentSeasonID()).Scan(&calibrationExists); err != nil || !calibrationExists {
			http.Error(w, "season_calibration_missing", http.StatusServiceUnavailable)
			return
		}

		heartbeat, err := readTickHeartbeat(ctx, db)
		if err != nil {
			http.Error(w, "tick_missing", http.StatusServiceUnavailable)
			return
		}
		maxAge := emissionTickInterval*2 + 10*time.Second
		if time.Since(heartbeat) > maxAge {
			http.Error(w, "tick_stale", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

func requireSession(db *sql.DB, w http.ResponseWriter, r *http.Request) (*Account, bool) {
	account, _, err := getSessionAccount(db, r)
	if err != nil || account == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, false
	}
	return account, true
}

func playerHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		if remainingCooldown, err := accountCooldownRemaining(db, account.AccountID, time.Now().UTC()); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		} else if remainingCooldown > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(remainingCooldown.Seconds())))
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "ACCOUNT_COOLDOWN", NextAvailableInSeconds: int64(remainingCooldown.Seconds())})
			return
		}
		if remaining, err := accountCooldownRemaining(db, account.AccountID, time.Now().UTC()); err != nil {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		} else if remaining > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(remaining.Seconds())))
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "ACCOUNT_COOLDOWN"})
			return
		}
		playerID := account.PlayerID

		player, err := LoadOrCreatePlayer(db, playerID)
		if err != nil {
			log.Println("Failed to load/create player:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if ip := getClientIP(r); ip != "" {
			isNew, err := RecordPlayerIP(db, playerID, ip)
			if err != nil {
				log.Println("Failed to record player IP:", err)
			} else if isNew {
				if err := ApplyIPDampeningDelay(db, playerID, ip); err != nil {
					log.Println("Failed to apply IP dampening delay:", err)
				}
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"playerCoins": player.Coins,
			"playerStars": player.Stars,
		})
	}
}

func seasonsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type SeasonView struct {
			SeasonID              string  `json:"seasonId"`
			SecondsRemaining      int64   `json:"secondsRemaining"`
			CoinsInCirculation    int64   `json:"coinsInCirculation"`
			CoinEmissionPerMinute float64 `json:"coinEmissionPerMinute"`
			CurrentStarPrice      int     `json:"currentStarPrice"`
		}

		now := time.Now().UTC()
		remaining := seasonSecondsRemaining(now)
		coins := economy.CoinsInCirculation()
		activeCoins := economy.ActiveCoinsInCirculation()
		emission := economy.EffectiveEmissionPerMinute(remaining, activeCoins)

		currentPrice := ComputeStarPrice(coins, remaining)
		if account, _, err := getSessionAccount(db, r); err == nil && account != nil {
			currentPrice = computePlayerStarPrice(db, account.PlayerID, coins, remaining)
		}

		response := []SeasonView{{
			SeasonID:              currentSeasonID(),
			SecondsRemaining:      remaining,
			CoinsInCirculation:    coins,
			CoinEmissionPerMinute: emission,
			CurrentStarPrice:      currentPrice,
		}}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"recommendedSeasonId": currentSeasonID(),
			"seasons":             response,
		})
	}
}

func computePlayerStarPrice(db *sql.DB, playerID string, coinsInCirculation int64, secondsRemaining int64) int {
	basePrice := ComputeStarPrice(coinsInCirculation, secondsRemaining)
	dampenedPrice, err := ComputeDampenedStarPrice(db, playerID, basePrice)
	if err != nil {
		return basePrice
	}
	enforcement := abuseEffectiveEnforcement(db, playerID, bulkStarMaxQty())
	return int(float64(dampenedPrice)*enforcement.PriceMultiplier + 0.9999)
}

func buyStarHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if isSeasonEnded(time.Now().UTC()) {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "SEASON_ENDED"})
			return
		}
		if !featureFlags.SinksEnabled {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "FEATURE_DISABLED"})
			return
		}

		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		if remainingCooldown, err := accountCooldownRemaining(db, account.AccountID, time.Now().UTC()); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		} else if remainingCooldown > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(remainingCooldown.Seconds())))
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "ACCOUNT_COOLDOWN", NextAvailableInSeconds: int64(remainingCooldown.Seconds())})
			return
		}
		if remaining, err := accountCooldownRemaining(db, account.AccountID, time.Now().UTC()); err != nil {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		} else if remaining > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(remaining.Seconds())))
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "ACCOUNT_COOLDOWN"})
			return
		}

		var req BuyStarRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(BuyStarResponse{
				OK: false, Error: "INVALID_REQUEST",
			})
			return
		}

		quantity := req.Quantity
		if quantity <= 0 {
			quantity = 1
		}

		playerID := account.PlayerID
		if !isValidPlayerID(playerID) {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		maxQty := abuseMaxBulkQty(db, playerID, bulkStarMaxQty())
		if quantity < 1 || quantity > maxQty {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INVALID_QUANTITY"})
			return
		}

		player, err := LoadPlayer(db, playerID)
		if err != nil || player == nil {
			json.NewEncoder(w).Encode(BuyStarResponse{
				OK: false, Error: "PLAYER_NOT_REGISTERED",
			})
			return
		}

		isBot, _, err := getPlayerBotInfo(db, playerID)
		if err != nil {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if isBot && !botsEnabled() {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "BOTS_DISABLED"})
			return
		}
		if isBot {
			allowed, retryAfter, err := enforceBotStarRateLimit(db, playerID, botMinStarInterval())
			if err != nil {
				json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "BOT_RATE_LIMIT"})
				return
			}
		}

		quote, err := buildBulkStarQuote(db, playerID, quantity)
		if err != nil {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer tx.Rollback()

		var coinsBefore int64
		var starsBefore int64
		var lastActive time.Time

		err = tx.QueryRowContext(r.Context(), `
			SELECT coins, stars, burned_coins, last_active_at
			FROM players
			WHERE player_id = $1
			FOR UPDATE
		`, playerID).Scan(&coinsBefore, &starsBefore, new(int64), &lastActive)

		if err != nil {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		if coinsBefore < quote.TotalCoinsSpent {
			activityWindow := ActiveActivityWindow()
			if time.Since(lastActive) <= activityWindow {
				emitServerTelemetryWithCooldown(db, &account.AccountID, playerID, "star_purchase_unaffordable_despite_activity", map[string]interface{}{
					"quantity":       quantity,
					"requiredCoins":  quote.TotalCoinsSpent,
					"playerCoins":    coinsBefore,
					"finalStarPrice": quote.FinalStarPrice,
					"activityWindow": int64(activityWindow.Seconds()),
				}, 10*time.Minute)
			}
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "NOT_ENOUGH_COINS"})
			return
		}

		coinsAfter := coinsBefore - quote.TotalCoinsSpent
		starsAfter := starsBefore + int64(quantity)

		_, err = tx.ExecContext(r.Context(), `
			UPDATE players
			SET coins = $2,
				stars = $3,
				burned_coins = burned_coins + $4,
				last_active_at = NOW()
			WHERE player_id = $1
		`, playerID, coinsAfter, starsAfter, quote.TotalCoinsSpent)

		if err != nil {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		purchaseType := "base"
		if quantity > 1 {
			purchaseType = "bulk"
		}
		runningCoins := coinsBefore
		runningStars := starsBefore
		for _, item := range quote.Breakdown {
			price := item.FinalPrice
			coinsAfterStep := runningCoins - int64(price)
			starsAfterStep := runningStars + 1
			if err := logStarPurchaseTx(
				tx,
				account.AccountID,
				playerID,
				currentSeasonID(),
				purchaseType,
				"",
				price,
				runningCoins,
				coinsAfterStep,
				runningStars,
				starsAfterStep,
			); err != nil {
				json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			runningCoins = coinsAfterStep
			runningStars = starsAfterStep
		}

		if err := tx.Commit(); err != nil {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		for i := 0; i < quantity; i++ {
			economy.IncrementStars()
		}
		lastPrice := 0
		if len(quote.Breakdown) > 0 {
			lastPrice = quote.Breakdown[len(quote.Breakdown)-1].FinalPrice
		}
		if !isBot {
			message := "Star purchased."
			if quantity > 1 {
				message = "Purchased " + strconv.Itoa(quantity) + " stars."
			}
			emitNotification(db, NotificationInput{
				RecipientRole:      NotificationRolePlayer,
				RecipientAccountID: account.AccountID,
				SeasonID:           currentSeasonID(),
				Category:           NotificationCategoryPlayerAction,
				Type:               "star_purchase",
				Priority:           NotificationPriorityNormal,
				Message:            message,
				Link:               "#/home",
				Payload: map[string]interface{}{
					"quantity":        quantity,
					"totalCoinsSpent": quote.TotalCoinsSpent,
					"finalStarPrice":  quote.FinalStarPrice,
					"playerCoins":     coinsAfter,
					"playerStars":     starsAfter,
				},
			})
			if quantity >= 3 {
				priority := NotificationPriorityNormal
				if quantity >= maxQty {
					priority = NotificationPriorityHigh
				}
				emitNotification(db, NotificationInput{
					RecipientRole: NotificationRoleAdmin,
					SeasonID:      currentSeasonID(),
					Category:      NotificationCategoryEconomy,
					Type:          "bulk_star_purchase",
					Priority:      priority,
					Message:       "Bulk star purchase: " + strconv.Itoa(quantity) + " stars.",
					Payload: map[string]interface{}{
						"accountId":       account.AccountID,
						"playerId":        playerID,
						"quantity":        quantity,
						"totalCoinsSpent": quote.TotalCoinsSpent,
						"finalStarPrice":  quote.FinalStarPrice,
					},
					DedupKey:    "bulk_star_purchase:" + account.AccountID + ":" + strconv.Itoa(quantity),
					DedupWindow: 10 * time.Minute,
				})
			}
		}
		json.NewEncoder(w).Encode(BuyStarResponse{
			OK:              true,
			StarPricePaid:   lastPrice,
			PlayerCoins:     int(coinsAfter),
			PlayerStars:     int(starsAfter),
			StarsPurchased:  quantity,
			TotalCoinsSpent: int(quote.TotalCoinsSpent),
			FinalStarPrice:  quote.FinalStarPrice,
			Breakdown:       quote.Breakdown,
			Warning:         quote.Warning,
			WarningLevel:    quote.WarningLevel,
		})
	}
}

func buyStarQuoteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if isSeasonEnded(time.Now().UTC()) {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "SEASON_ENDED"})
			return
		}
		if !featureFlags.SinksEnabled {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "FEATURE_DISABLED"})
			return
		}
		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		if remaining, err := accountCooldownRemaining(db, account.AccountID, time.Now().UTC()); err != nil {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		} else if remaining > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(remaining.Seconds())))
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "ACCOUNT_COOLDOWN"})
			return
		}

		var req BuyStarRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		quantity := req.Quantity
		if quantity <= 0 {
			quantity = 1
		}

		playerID := account.PlayerID
		if !isValidPlayerID(playerID) {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		maxQty := abuseMaxBulkQty(db, playerID, bulkStarMaxQty())
		if quantity < 1 || quantity > maxQty {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "INVALID_QUANTITY"})
			return
		}

		player, err := LoadPlayer(db, playerID)
		if err != nil || player == nil {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "PLAYER_NOT_REGISTERED"})
			return
		}

		isBot, _, err := getPlayerBotInfo(db, playerID)
		if err != nil {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if isBot && !botsEnabled() {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "BOTS_DISABLED"})
			return
		}

		quote, err := buildBulkStarQuote(db, playerID, quantity)
		if err != nil {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		shortfall := int(quote.TotalCoinsSpent - player.Coins)
		if shortfall < 0 {
			shortfall = 0
		}

		json.NewEncoder(w).Encode(BuyStarQuoteResponse{
			OK:              true,
			StarsRequested:  quantity,
			TotalCoinsSpent: int(quote.TotalCoinsSpent),
			FinalStarPrice:  quote.FinalStarPrice,
			Breakdown:       quote.Breakdown,
			Warning:         quote.Warning,
			WarningLevel:    quote.WarningLevel,
			CanAfford:       player.Coins >= quote.TotalCoinsSpent,
			Shortfall:       shortfall,
		})
	}
}

type bulkStarQuote struct {
	TotalCoinsSpent int64
	FinalStarPrice  int
	Breakdown       []BulkStarBreakdown
	Warning         string
	WarningLevel    string
}

func buildBulkStarQuote(db *sql.DB, playerID string, quantity int) (bulkStarQuote, error) {
	if quantity < 1 {
		return bulkStarQuote{}, nil
	}
	enforcement := abuseEffectiveEnforcement(db, playerID, bulkStarMaxQty())
	coinsInCirculation := economy.CoinsInCirculation()
	secondsRemaining := seasonSecondsRemaining(time.Now().UTC())
	baseStars := economy.StarsPurchased()
	gamma := bulkStarGamma()

	breakdown := make([]BulkStarBreakdown, 0, quantity)
	var total int64
	maxMultiplier := 1.0
	for i := 0; i < quantity; i++ {
		basePrice := ComputeStarPriceWithStars(baseStars+i, coinsInCirculation, secondsRemaining)
		dampenedPrice, err := ComputeDampenedStarPrice(db, playerID, basePrice)
		if err != nil {
			return bulkStarQuote{}, err
		}
		multiplier := 1 + gamma*float64(i*i)
		if multiplier > maxMultiplier {
			maxMultiplier = multiplier
		}
		finalPrice := int(float64(dampenedPrice)*multiplier*enforcement.PriceMultiplier + 0.9999)
		breakdown = append(breakdown, BulkStarBreakdown{
			Index:          i + 1,
			BasePrice:      dampenedPrice,
			BulkMultiplier: multiplier,
			FinalPrice:     finalPrice,
		})
		total += int64(finalPrice)
	}
	finalStarPrice := 0
	if len(breakdown) > 0 {
		finalStarPrice = breakdown[len(breakdown)-1].FinalPrice
	}
	warning, warningLevel := bulkWarning(maxMultiplier)
	return bulkStarQuote{
		TotalCoinsSpent: total,
		FinalStarPrice:  finalStarPrice,
		Breakdown:       breakdown,
		Warning:         warning,
		WarningLevel:    warningLevel,
	}, nil
}

func bulkStarMaxQty() int {
	const fallback = 5
	value := strings.TrimSpace(os.Getenv("BULK_STAR_MAX_QTY"))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func bulkStarGamma() float64 {
	return economy.Calibration().Gamma
}

func bulkWarning(maxMultiplier float64) (string, string) {
	if maxMultiplier >= 5 {
		return "Severe bulk penalty. Late-season bulk buys are catastrophic.", "severe"
	}
	if maxMultiplier >= 3 {
		return "Heavy bulk penalty. Bulk purchases are highly inefficient.", "high"
	}
	if maxMultiplier >= 2 {
		return "Bulk penalty rising. Consider smaller buys.", "medium"
	}
	return "", ""
}

func buyVariantStarHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if isSeasonEnded(time.Now().UTC()) {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "SEASON_ENDED"})
			return
		}
		if !featureFlags.SinksEnabled {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "FEATURE_DISABLED"})
			return
		}

		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		if remaining, err := accountCooldownRemaining(db, account.AccountID, time.Now().UTC()); err != nil {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		} else if remaining > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(remaining.Seconds())))
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "ACCOUNT_COOLDOWN"})
			return
		}

		var req BuyVariantStarRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		playerID := account.PlayerID
		if !isValidPlayerID(playerID) {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		variantMultiplier := 0.0
		switch req.Variant {
		case StarVariantEmber:
			variantMultiplier = 2.0
		case StarVariantVoid:
			variantMultiplier = 4.0
		default:
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "INVALID_VARIANT"})
			return
		}

		player, err := LoadPlayer(db, playerID)
		if err != nil || player == nil {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "PLAYER_NOT_REGISTERED"})
			return
		}

		isBot, _, err := getPlayerBotInfo(db, playerID)
		if err != nil {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if isBot && !botsEnabled() {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "BOTS_DISABLED"})
			return
		}
		if isBot {
			allowed, retryAfter, err := enforceBotStarRateLimit(db, playerID, botMinStarInterval())
			if err != nil {
				json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "BOT_RATE_LIMIT"})
				return
			}
		}

		coinsBefore := player.Coins
		starsBefore := player.Stars

		basePrice := ComputeStarPrice(
			economy.CoinsInCirculation(),
			28*24*3600,
		)

		dampenedPrice, err := ComputeDampenedStarPrice(db, playerID, basePrice)
		if err != nil {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		enforcement := abuseEffectiveEnforcement(db, playerID, bulkStarMaxQty())
		baseVariantPrice := int(float64(dampenedPrice)*variantMultiplier + 0.9999)
		price := abuseAdjustedPrice(baseVariantPrice, enforcement.PriceMultiplier)
		if player.Coins < int64(price) {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "NOT_ENOUGH_COINS"})
			return
		}

		player.Coins -= int64(price)
		if err := UpdatePlayerBalances(db, player.PlayerID, player.Coins, player.Stars); err != nil {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if err := AddStarVariant(db, player.PlayerID, req.Variant, 1); err != nil {
			json.NewEncoder(w).Encode(BuyVariantStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		logStarPurchase(
			db,
			account.AccountID,
			player.PlayerID,
			currentSeasonID(),
			"variant",
			req.Variant,
			price,
			coinsBefore,
			player.Coins,
			starsBefore,
			player.Stars,
		)

		json.NewEncoder(w).Encode(BuyVariantStarResponse{
			OK:          true,
			Variant:     req.Variant,
			PricePaid:   price,
			PlayerCoins: int(player.Coins),
		})
	}
}

func buyBoostHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if isSeasonEnded(time.Now().UTC()) {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "SEASON_ENDED"})
			return
		}
		if !featureFlags.SinksEnabled {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "FEATURE_DISABLED"})
			return
		}

		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		if remaining, err := accountCooldownRemaining(db, account.AccountID, time.Now().UTC()); err != nil {
			json.NewEncoder(w).Encode(BurnCoinsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		} else if remaining > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(remaining.Seconds())))
			json.NewEncoder(w).Encode(BurnCoinsResponse{OK: false, Error: "ACCOUNT_COOLDOWN"})
			return
		}

		var req BuyBoostRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		playerID := account.PlayerID
		if !isValidPlayerID(playerID) {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		if req.BoostType != BoostActivity {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "INVALID_BOOST"})
			return
		}

		player, err := LoadPlayer(db, playerID)
		if err != nil || player == nil {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "PLAYER_NOT_REGISTERED"})
			return
		}

		const price = 25
		const duration = 30 * time.Minute
		enforcement := abuseEffectiveEnforcement(db, playerID, bulkStarMaxQty())
		finalPrice := abuseAdjustedPrice(price, enforcement.PriceMultiplier)
		throttled, err := IsPlayerThrottledByIP(db, playerID)
		if err != nil {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if throttled {
			trustStatus, err := accountTrustStatusForPlayer(db, playerID)
			if err != nil {
				trustStatus = trustStatusNormal
			}
			multiplier := ipDampeningPriceMultiplier * trustStatusPriceMultiplier(trustStatus)
			finalPrice = int(float64(finalPrice)*multiplier + 0.9999)
		}

		if player.Coins < int64(finalPrice) {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "NOT_ENOUGH_COINS"})
			return
		}

		player.Coins -= int64(finalPrice)
		if err := UpdatePlayerBalances(db, player.PlayerID, player.Coins, player.Stars); err != nil {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		expiresAt, err := SetBoost(db, player.PlayerID, BoostActivity, duration)
		if err != nil {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		json.NewEncoder(w).Encode(BuyBoostResponse{
			OK:          true,
			BoostType:   BoostActivity,
			ExpiresAt:   expiresAt,
			PlayerCoins: int(player.Coins),
		})
	}
}

func burnCoinsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if isSeasonEnded(time.Now().UTC()) {
			json.NewEncoder(w).Encode(BurnCoinsResponse{OK: false, Error: "SEASON_ENDED"})
			return
		}
		if !featureFlags.SinksEnabled {
			json.NewEncoder(w).Encode(BurnCoinsResponse{OK: false, Error: "FEATURE_DISABLED"})
			return
		}

		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}

		var req BurnCoinsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(BurnCoinsResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		playerID := account.PlayerID
		if !isValidPlayerID(playerID) {
			json.NewEncoder(w).Encode(BurnCoinsResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		if req.Amount <= 0 || req.Amount > 1000 {
			json.NewEncoder(w).Encode(BurnCoinsResponse{OK: false, Error: "INVALID_AMOUNT"})
			return
		}

		result, err := db.Exec(`
			UPDATE players
			SET coins = coins - $2,
			    burned_coins = burned_coins + $2
			WHERE player_id = $1 AND coins >= $2
		`, playerID, req.Amount)
		if err != nil {
			json.NewEncoder(w).Encode(BurnCoinsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			json.NewEncoder(w).Encode(BurnCoinsResponse{OK: false, Error: "NOT_ENOUGH_COINS"})
			return
		}

		var coins int
		var burned int
		err = db.QueryRow(`
			SELECT coins, burned_coins
			FROM players
			WHERE player_id = $1
		`, playerID).Scan(&coins, &burned)
		if err != nil {
			json.NewEncoder(w).Encode(BurnCoinsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		json.NewEncoder(w).Encode(BurnCoinsResponse{
			OK:          true,
			Amount:      req.Amount,
			PlayerCoins: coins,
			BurnedTotal: burned,
		})
	}
}

func requestPasswordResetHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req PasswordResetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Identifier == "" {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		account, err := lookupAccountForReset(db, req.Identifier)
		if err == sql.ErrNoRows {
			json.NewEncoder(w).Encode(SimpleResponse{OK: true})
			return
		}
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if account.Email == "" {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "EMAIL_NOT_SET"})
			return
		}
		token, err := createPasswordResetToken(db, account.AccountID)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		baseURL := os.Getenv("APP_BASE_URL")
		if baseURL == "" {
			scheme := "https"
			if r.Header.Get("X-Forwarded-Proto") != "" {
				scheme = r.Header.Get("X-Forwarded-Proto")
			} else if r.TLS == nil {
				scheme = "http"
			}
			baseURL = scheme + "://" + r.Host
		}
		if err := sendPasswordResetEmail(account.Email, token, baseURL); err != nil {
			if err.Error() == "EMAIL_NOT_CONFIGURED" {
				json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "EMAIL_NOT_CONFIGURED"})
				return
			}
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "EMAIL_SEND_FAILED"})
			return
		}
		json.NewEncoder(w).Encode(SimpleResponse{OK: true})
	}
}

func resetPasswordHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req PasswordResetConfirmRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" || req.NewPassword == "" {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		err := resetPasswordWithToken(db, req.Token, req.NewPassword)
		if err != nil {
			switch err.Error() {
			case "INVALID_PASSWORD", "INVALID_TOKEN", "TOKEN_USED", "TOKEN_EXPIRED":
				json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: err.Error()})
				return
			default:
				json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
		}
		json.NewEncoder(w).Encode(SimpleResponse{OK: true})
	}
}

func bootstrapPasswordHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req BootstrapPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		username := strings.ToLower(strings.TrimSpace(req.Username))
		newPassword := strings.TrimSpace(req.NewPassword)
		gateKey := strings.TrimSpace(req.GateKey)
		if username == "" || newPassword == "" || gateKey == "" {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		if len(newPassword) < 8 || len(newPassword) > 128 {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_PASSWORD"})
			return
		}

		ctx := r.Context()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer tx.Rollback()

		var accountID string
		var role string
		var mustChange bool
		var passwordHash string
		if err := tx.QueryRowContext(ctx, `
			SELECT account_id, role, must_change_password, password_hash
			FROM accounts
			WHERE username = $1
			FOR UPDATE
		`, username).Scan(&accountID, &role, &mustChange, &passwordHash); err != nil {
			if err == sql.ErrNoRows {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if normalizeRole(role) != "admin" || !mustChange {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if verifyPassword(passwordHash, newPassword) {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "PASSWORD_REUSE"})
			return
		}

		var gateID int64
		if err := tx.QueryRowContext(ctx, `
			SELECT gate_id
			FROM admin_password_gates
			WHERE account_id = $1
				AND gate_key = $2
				AND used_at IS NULL
			FOR UPDATE
		`, accountID, gateKey).Scan(&gateID); err != nil {
			if err == sql.ErrNoRows {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		newHash, err := hashPassword(newPassword)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE accounts
			SET password_hash = $2,
				must_change_password = FALSE
			WHERE account_id = $1
		`, accountID, newHash); err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		ip := getClientIP(r)
		if _, err := tx.ExecContext(ctx, `
			UPDATE admin_password_gates
			SET used_at = NOW(),
				used_by_ip = $2
			WHERE gate_id = $1
		`, gateID, ip); err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		details := map[string]interface{}{
			"accountId": accountID,
			"ip":        ip,
		}
		payload, err := json.Marshal(details)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO admin_audit_log (admin_account_id, action_type, scope_type, scope_id, reason, details, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
		`, accountID, "bootstrap_password_change", "account", accountID, "bootstrap", string(payload)); err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		if err := tx.Commit(); err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		json.NewEncoder(w).Encode(SimpleResponse{OK: true})
	}
}

func notificationsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		query := r.URL.Query()
		afterID := int64(0)
		if raw := strings.TrimSpace(query.Get("after")); raw != "" {
			if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
				afterID = parsed
			}
		}
		limit := 60
		if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		role := notificationRoleForAccount(account)
		items, err := fetchNotifications(db, account.AccountID, role, afterID, limit, afterID > 0)
		if err != nil {
			json.NewEncoder(w).Encode(NotificationsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		json.NewEncoder(w).Encode(NotificationsResponse{OK: true, Notifications: items})
	}
}

func notificationsAckHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		var req NotificationAckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		role := notificationRoleForAccount(account)
		for _, id := range req.IDs {
			_, _ = db.Exec(`
				INSERT INTO notification_reads (notification_id, account_id, read_at)
				SELECT n.id, $1, NOW()
				FROM notifications n
				LEFT JOIN notification_deletes d
					ON d.notification_id = n.id AND d.account_id = $1
				WHERE n.id = $3
					AND d.notification_id IS NULL
`+notificationAccessSQL+`
				ON CONFLICT (notification_id, account_id) DO NOTHING
			`, account.AccountID, role, id)

			_, _ = db.Exec(`
				INSERT INTO notification_acks (notification_id, account_id, acknowledged_at)
				SELECT n.id, $1, NOW()
				FROM notifications n
				LEFT JOIN notification_deletes d
					ON d.notification_id = n.id AND d.account_id = $1
				WHERE n.id = $3
					AND d.notification_id IS NULL
					AND (n.ack_required OR COALESCE(n.priority, 'normal') <> 'normal')
`+notificationAccessSQL+`
				ON CONFLICT (notification_id, account_id) DO NOTHING
			`, account.AccountID, role, id)
		}
		json.NewEncoder(w).Encode(SimpleResponse{OK: true})
	}
}

func notificationsDeleteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		var req NotificationDeleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		role := notificationRoleForAccount(account)
		for _, id := range req.IDs {
			_, _ = db.Exec(`
				INSERT INTO notification_deletes (notification_id, account_id, deleted_at)
				SELECT n.id, $1, NOW()
				FROM notifications n
				WHERE n.id = $3
`+notificationAccessSQL+`
				ON CONFLICT (notification_id, account_id) DO NOTHING
			`, account.AccountID, role, id)
		}
		json.NewEncoder(w).Encode(SimpleResponse{OK: true})
	}
}

func notificationsSettingsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		switch r.Method {
		case http.MethodGet:
			rows, err := db.Query(`
				SELECT category, enabled, push_enabled
				FROM notification_settings
				WHERE account_id = $1
			`, account.AccountID)
			if err != nil {
				json.NewEncoder(w).Encode(NotificationSettingsResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			defer rows.Close()
			settings := map[string]bool{}
			pushSettings := map[string]bool{}
			for rows.Next() {
				var category string
				var enabled bool
				var pushEnabled bool
				if err := rows.Scan(&category, &enabled, &pushEnabled); err != nil {
					continue
				}
				settings[category] = enabled
				pushSettings[category] = pushEnabled
			}
			items := []NotificationSettingItem{}
			for _, category := range NotificationCategories() {
				enabled := true
				pushEnabled := false
				if value, ok := settings[category]; ok {
					enabled = value
				}
				if value, ok := pushSettings[category]; ok {
					pushEnabled = value
				}
				items = append(items, NotificationSettingItem{Category: category, Enabled: enabled, PushEnabled: pushEnabled})
			}
			json.NewEncoder(w).Encode(NotificationSettingsResponse{OK: true, Settings: items})
			return
		case http.MethodPost:
			var req NotificationSettingsUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Categories) == 0 {
				json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_REQUEST"})
				return
			}
			for _, item := range req.Categories {
				category := normalizeNotificationCategory(item.Category)
				_, _ = db.Exec(`
					INSERT INTO notification_settings (account_id, category, enabled, push_enabled, updated_at)
					VALUES ($1, $2, $3, $4, NOW())
					ON CONFLICT (account_id, category)
					DO UPDATE SET enabled = EXCLUDED.enabled, push_enabled = EXCLUDED.push_enabled, updated_at = NOW()
				`, account.AccountID, category, item.Enabled, item.PushEnabled)
			}
			json.NewEncoder(w).Encode(SimpleResponse{OK: true})
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}
}

func notificationsStreamHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		lastID := int64(0)
		if raw := strings.TrimSpace(r.Header.Get("Last-Event-ID")); raw != "" {
			if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
				lastID = parsed
			}
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("after")); raw != "" {
			if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > lastID {
				lastID = parsed
			}
		}

		role := notificationRoleForAccount(account)
		ticker := time.NewTicker(3 * time.Second)
		heartbeat := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		defer heartbeat.Stop()

		flusher.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-heartbeat.C:
				_, _ = w.Write([]byte(": ping\n\n"))
				flusher.Flush()
			case <-ticker.C:
				items, err := fetchNotifications(db, account.AccountID, role, lastID, 25, true)
				if err != nil {
					continue
				}
				for _, item := range items {
					payload, err := json.Marshal(item)
					if err != nil {
						continue
					}
					if _, err := w.Write([]byte("id: " + strconv.FormatInt(item.ID, 10) + "\n")); err != nil {
						return
					}
					if _, err := w.Write([]byte("event: notification\n")); err != nil {
						return
					}
					if _, err := w.Write([]byte("data: ")); err != nil {
						return
					}
					if _, err := w.Write(payload); err != nil {
						return
					}
					if _, err := w.Write([]byte("\n\n")); err != nil {
						return
					}
					flusher.Flush()
					if item.ID > lastID {
						lastID = item.ID
					}
				}
			}
		}
	}
}

func activityHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		_, err := db.Exec(`
			UPDATE players
			SET last_active_at = NOW()
			WHERE player_id = $1
		`, account.PlayerID)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		json.NewEncoder(w).Encode(SimpleResponse{OK: true})
	}
}

func signupHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req SignupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		ip := getClientIP(r)
		limit, window := authRateLimitConfig("signup")
		allowedRate, retryAfter, err := checkAuthRateLimit(db, ip, "signup", limit, window)
		if err != nil {
			log.Println("signup: rate limit error:", err)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !allowedRate {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "RATE_LIMIT"})
			return
		}
		log.Printf("signup: whitelist gating removed; relying on throttles (ip=%s)", ip)
		account, err := createAccount(db, req.Username, req.Password, req.DisplayName, req.Email)
		if err != nil {
			log.Println("signup: createAccount error:", err)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: err.Error()})
			return
		}
		if ip != "" {
			isNew, err := RecordPlayerIP(db, account.PlayerID, ip)
			if err == nil && isNew {
				_ = ApplyIPDampeningDelay(db, account.PlayerID, ip)
			}
		}
		if _, err := LoadOrCreatePlayer(db, account.PlayerID); err != nil {
			log.Println("signup: LoadOrCreatePlayer error:", err)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		EnsurePlayableBalanceOnLogin(db, account.PlayerID, &account.AccountID)
		verifyDailyPlayability(db, account.PlayerID, &account.AccountID)
		emitNotification(db, NotificationInput{
			RecipientRole:      NotificationRolePlayer,
			RecipientAccountID: account.AccountID,
			Category:           NotificationCategorySystem,
			Type:               "welcome",
			Priority:           NotificationPriorityNormal,
			Message:            "Welcome to the season. Prices rise over time, but small goals still stack. Track the curve and set a first-star target—no outcome is guaranteed.",
			Link:               "#/home",
		})
		sessionID, expiresAt, err := createSession(db, account.AccountID)
		if err != nil {
			log.Println("signup: createSession error:", err)
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		writeSessionCookie(w, sessionID, expiresAt)

		accessToken, accessExpires, err := issueAccessToken(account.AccountID, accessTokenTTL)
		if err != nil {
			log.Println("signup: issueAccessToken error:", err)
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		refreshToken, _, err := createRefreshToken(db, account.AccountID, "auth", r.UserAgent(), getClientIP(r))
		if err != nil {
			log.Println("signup: createRefreshToken error:", err)
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		json.NewEncoder(w).Encode(AuthResponse{
			OK:                 true,
			Username:           account.Username,
			DisplayName:        account.DisplayName,
			PlayerID:           account.PlayerID,
			Role:               account.Role,
			MustChangePassword: account.MustChangePassword,
			AccessToken:        accessToken,
			RefreshToken:       refreshToken,
			ExpiresIn:          int64(time.Until(accessExpires).Seconds()),
		})
	}
}

func loginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		ip := getClientIP(r)
		limit, window := authRateLimitConfig("login")
		allowedRate, retryAfter, err := checkAuthRateLimit(db, ip, "login", limit, window)
		if err != nil {
			log.Println("login: rate limit error:", err)
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !allowedRate {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "RATE_LIMIT"})
			return
		}

		account, err := authenticate(db, req.Username, req.Password)
		if err != nil {
			if err.Error() == "ACCOUNT_FROZEN" {
				json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "ACCOUNT_FROZEN"})
				return
			}
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INVALID_CREDENTIALS"})
			return
		}

		if _, err := LoadOrCreatePlayer(db, account.PlayerID); err != nil {
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		EnsurePlayableBalanceOnLogin(db, account.PlayerID, &account.AccountID)
		verifyDailyPlayability(db, account.PlayerID, &account.AccountID)

		sessionID, expiresAt, err := createSession(db, account.AccountID)
		if err != nil {
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		writeSessionCookie(w, sessionID, expiresAt)

		accessToken, accessExpires, err := issueAccessToken(account.AccountID, accessTokenTTL)
		if err != nil {
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		refreshToken, _, err := createRefreshToken(db, account.AccountID, "auth", r.UserAgent(), getClientIP(r))
		if err != nil {
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		json.NewEncoder(w).Encode(AuthResponse{
			OK:                 true,
			Username:           account.Username,
			DisplayName:        account.DisplayName,
			PlayerID:           account.PlayerID,
			IsAdmin:            account.Role == "admin",
			IsModerator:        account.Role == "moderator",
			Role:               account.Role,
			MustChangePassword: account.MustChangePassword,
			AccessToken:        accessToken,
			RefreshToken:       refreshToken,
			ExpiresIn:          int64(time.Until(accessExpires).Seconds()),
		})
	}
}

func logoutHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		_, sessionID, err := getSessionAccount(db, r)
		if err == nil && sessionID != "" {
			clearSession(db, sessionID)
		}
		clearSessionCookie(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

func meHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, _, err := getSessionAccount(db, r)
		if err != nil || account == nil {
			json.NewEncoder(w).Encode(AuthResponse{OK: false})
			return
		}
		EnsurePlayableBalanceOnLogin(db, account.PlayerID, &account.AccountID)
		verifyDailyPlayability(db, account.PlayerID, &account.AccountID)
		json.NewEncoder(w).Encode(AuthResponse{
			OK:                 true,
			Username:           account.Username,
			DisplayName:        account.DisplayName,
			PlayerID:           account.PlayerID,
			IsAdmin:            account.Role == "admin",
			IsModerator:        account.Role == "moderator",
			Role:               account.Role,
			MustChangePassword: account.MustChangePassword,
		})
	}
}

func refreshTokenHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var token string
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			token = strings.TrimSpace(auth[len("bearer "):])
		}
		if token == "" {
			var req RefreshTokenRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			token = strings.TrimSpace(req.RefreshToken)
		}
		if token == "" {
			json.NewEncoder(w).Encode(RefreshTokenResponse{OK: false, Error: "MISSING_REFRESH_TOKEN"})
			return
		}

		accessToken, accessExpires, refreshToken, _, err := rotateRefreshToken(db, token, r.UserAgent(), getClientIP(r))
		if err != nil {
			json.NewEncoder(w).Encode(RefreshTokenResponse{OK: false, Error: err.Error()})
			return
		}

		json.NewEncoder(w).Encode(RefreshTokenResponse{
			OK:           true,
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    int64(time.Until(accessExpires).Seconds()),
		})
	}
}

func profileHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}

		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(ProfileResponse{
				OK:          true,
				Username:    account.Username,
				DisplayName: account.DisplayName,
				Email:       account.Email,
				Bio:         account.Bio,
				Pronouns:    account.Pronouns,
				Location:    account.Location,
				Website:     account.Website,
				AvatarURL:   account.AvatarURL,
			})
			return
		case http.MethodPost:
			var req ProfileUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				json.NewEncoder(w).Encode(ProfileResponse{OK: false, Error: "INVALID_REQUEST"})
				return
			}
			displayName := strings.TrimSpace(req.DisplayName)
			if displayName == "" || len(displayName) > 32 {
				json.NewEncoder(w).Encode(ProfileResponse{OK: false, Error: "INVALID_DISPLAY_NAME"})
				return
			}
			bio := strings.TrimSpace(req.Bio)
			pronouns := strings.TrimSpace(req.Pronouns)
			location := strings.TrimSpace(req.Location)
			website := strings.TrimSpace(req.Website)
			avatarURL := strings.TrimSpace(req.AvatarURL)
			if len(bio) > 240 || len(pronouns) > 32 || len(location) > 48 || len(website) > 200 || len(avatarURL) > 200 {
				json.NewEncoder(w).Encode(ProfileResponse{OK: false, Error: "INVALID_PROFILE_FIELD"})
				return
			}
			normalizedEmail := ""
			if strings.TrimSpace(req.Email) != "" {
				var err error
				normalizedEmail, err = normalizeEmail(req.Email)
				if err != nil {
					json.NewEncoder(w).Encode(ProfileResponse{OK: false, Error: "INVALID_EMAIL"})
					return
				}
			}

			emailValue := sql.NullString{String: normalizedEmail, Valid: normalizedEmail != ""}
			_, err := db.Exec(`
				UPDATE accounts
				SET display_name = $2,
					email = $3,
					bio = $4,
					pronouns = $5,
					location = $6,
					website = $7,
					avatar_url = $8
				WHERE account_id = $1
			`, account.AccountID, displayName, emailValue, bio, pronouns, location, website, avatarURL)
			if err != nil {
				json.NewEncoder(w).Encode(ProfileResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			account.Email = normalizedEmail
			account.Bio = bio
			account.Pronouns = pronouns
			account.Location = location
			account.Website = website
			account.AvatarURL = avatarURL

			json.NewEncoder(w).Encode(ProfileResponse{
				OK:          true,
				Username:    account.Username,
				DisplayName: displayName,
				Email:       account.Email,
				Bio:         account.Bio,
				Pronouns:    account.Pronouns,
				Location:    account.Location,
				Website:     account.Website,
				AvatarURL:   account.AvatarURL,
			})
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}
}

func dailyClaimHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if isSeasonEnded(time.Now().UTC()) {
			emitServerTelemetryWithCooldown(db, nil, "", "faucet_denied", map[string]interface{}{
				"faucet": FaucetDaily,
				"reason": "SEASON_ENDED",
			}, 5*time.Minute)
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "SEASON_ENDED"})
			return
		}
		if !featureFlags.FaucetsEnabled {
			emitServerTelemetryWithCooldown(db, nil, "", "faucet_denied", map[string]interface{}{
				"faucet": FaucetDaily,
				"reason": "FEATURE_DISABLED",
			}, 5*time.Minute)
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "FEATURE_DISABLED"})
			return
		}

		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		playerID := account.PlayerID
		if !isValidPlayerID(playerID) {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		var req FaucetClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		player, err := LoadPlayer(db, playerID)
		if err != nil || player == nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "PLAYER_NOT_REGISTERED"})
			return
		}

		params := economy.Calibration()
		reward := params.DailyLoginReward
		cooldown := time.Duration(params.DailyLoginCooldownHours) * time.Hour
		enforcement := abuseEffectiveEnforcement(db, playerID, bulkStarMaxQty())
		reward = abuseAdjustedReward(reward, enforcement.EarnMultiplier)
		cooldown += abuseCooldownJitter(cooldown, enforcement.CooldownJitterFactor)
		scaling := currentFaucetScaling(time.Now().UTC())
		reward = applyFaucetRewardScaling(reward, scaling.RewardMultiplier)
		cooldown = applyFaucetCooldownScaling(cooldown, scaling.CooldownMultiplier)
		reward, err = ApplyIPDampeningReward(db, playerID, reward)
		if err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		canClaim, remaining, err := CanClaimFaucet(db, playerID, FaucetDaily, cooldown)
		if err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !canClaim {
			logFaucetDenied(db, &account.AccountID, playerID, FaucetDaily, "COOLDOWN", map[string]interface{}{
				"nextAvailableInSeconds": int64(remaining.Seconds()),
			})
			json.NewEncoder(w).Encode(FaucetClaimResponse{
				OK:                     false,
				Error:                  "COOLDOWN",
				NextAvailableInSeconds: int64(remaining.Seconds()),
			})
			return
		}

		remainingCap, err := RemainingDailyCap(db, playerID, time.Now().UTC())
		if err != nil {
			logFaucetDenied(db, &account.AccountID, playerID, FaucetDaily, "INTERNAL_ERROR", map[string]interface{}{
				"stage": "remaining_cap",
			})
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if remainingCap <= 0 {
			logFaucetDenied(db, &account.AccountID, playerID, FaucetDaily, "DAILY_CAP", nil)
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "DAILY_CAP"})
			return
		}
		if reward > remainingCap {
			reward = remainingCap
		}

		adjustedReward, ok := TryDistributeCoinsWithPriority(FaucetDaily, reward)
		if !ok {
			logFaucetDenied(db, &account.AccountID, playerID, FaucetDaily, "EMISSION_EXHAUSTED", map[string]interface{}{
				"attempted":      reward,
				"availableCoins": economy.AvailableCoins(),
			})
			emitNotification(db, NotificationInput{
				RecipientRole: NotificationRoleAdmin,
				Category:      NotificationCategoryEconomy,
				Type:          "emission_exhausted_daily",
				Priority:      NotificationPriorityHigh,
				Message:       "Daily faucet blocked by emission pool exhaustion.",
				Payload: map[string]interface{}{
					"playerId":  playerID,
					"attempted": reward,
				},
				DedupKey:    "emission_exhausted_daily",
				DedupWindow: 30 * time.Minute,
			})
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "EMISSION_EXHAUSTED"})
			return
		}
		reward = adjustedReward

		granted, _, err := GrantCoinsWithCap(db, player.PlayerID, reward, time.Now().UTC(), FaucetDaily, &account.AccountID)
		if err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if err := RecordFaucetClaim(db, player.PlayerID, FaucetDaily); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		player.Coins += int64(granted)
		emitServerTelemetry(db, &account.AccountID, playerID, "faucet_claim", map[string]interface{}{
			"faucet":        FaucetDaily,
			"granted":       granted,
			"attempted":     reward,
			"playerCoins":   player.Coins,
			"remainingCap":  remainingCap,
			"availableCoins": economy.AvailableCoins(),
		})

		json.NewEncoder(w).Encode(FaucetClaimResponse{
			OK:          true,
			Reward:      granted,
			PlayerCoins: int(player.Coins),
		})
	}
}

func activityClaimHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if isSeasonEnded(time.Now().UTC()) {
			emitServerTelemetryWithCooldown(db, nil, "", "faucet_denied", map[string]interface{}{
				"faucet": FaucetActivity,
				"reason": "SEASON_ENDED",
			}, 5*time.Minute)
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "SEASON_ENDED"})
			return
		}
		if !featureFlags.FaucetsEnabled {
			emitServerTelemetryWithCooldown(db, nil, "", "faucet_denied", map[string]interface{}{
				"faucet": FaucetActivity,
				"reason": "FEATURE_DISABLED",
			}, 5*time.Minute)
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "FEATURE_DISABLED"})
			return
		}

		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		playerID := account.PlayerID
		if !isValidPlayerID(playerID) {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		var req FaucetClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		player, err := LoadPlayer(db, playerID)
		if err != nil || player == nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "PLAYER_NOT_REGISTERED"})
			return
		}

		params := economy.Calibration()
		reward := params.ActivityReward
		cooldown := time.Duration(params.ActivityCooldownSeconds) * time.Second

		if active, _, err := HasActiveBoost(db, playerID, BoostActivity); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		} else if active {
			reward += 1
		}
		enforcement := abuseEffectiveEnforcement(db, playerID, bulkStarMaxQty())
		reward = abuseAdjustedReward(reward, enforcement.EarnMultiplier)
		cooldown += abuseCooldownJitter(cooldown, enforcement.CooldownJitterFactor)
		scaling := currentFaucetScaling(time.Now().UTC())
		reward = applyFaucetRewardScaling(reward, scaling.RewardMultiplier)
		cooldown = applyFaucetCooldownScaling(cooldown, scaling.CooldownMultiplier)
		reward, err = ApplyIPDampeningReward(db, playerID, reward)
		if err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		canClaim, remaining, err := CanClaimFaucet(db, playerID, FaucetActivity, cooldown)
		if err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !canClaim {
			logFaucetDenied(db, &account.AccountID, playerID, FaucetActivity, "COOLDOWN", map[string]interface{}{
				"nextAvailableInSeconds": int64(remaining.Seconds()),
			})
			json.NewEncoder(w).Encode(FaucetClaimResponse{
				OK:                     false,
				Error:                  "COOLDOWN",
				NextAvailableInSeconds: int64(remaining.Seconds()),
			})
			return
		}

		remainingCap, err := RemainingDailyCap(db, playerID, time.Now().UTC())
		if err != nil {
			logFaucetDenied(db, &account.AccountID, playerID, FaucetActivity, "INTERNAL_ERROR", map[string]interface{}{
				"stage": "remaining_cap",
			})
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if remainingCap <= 0 {
			logFaucetDenied(db, &account.AccountID, playerID, FaucetActivity, "DAILY_CAP", nil)
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "DAILY_CAP"})
			return
		}
		if reward > remainingCap {
			reward = remainingCap
		}
		adjustedReward, ok := TryDistributeCoinsWithPriority(FaucetActivity, reward)
		if !ok {
			logFaucetDenied(db, &account.AccountID, playerID, FaucetActivity, "EMISSION_EXHAUSTED", map[string]interface{}{
				"attempted":      reward,
				"availableCoins": economy.AvailableCoins(),
			})
			emitNotification(db, NotificationInput{
				RecipientRole: NotificationRoleAdmin,
				Category:      NotificationCategoryEconomy,
				Type:          "emission_exhausted_activity",
				Priority:      NotificationPriorityHigh,
				Message:       "Activity faucet blocked by emission pool exhaustion.",
				Payload: map[string]interface{}{
					"playerId":  playerID,
					"attempted": reward,
				},
				DedupKey:    "emission_exhausted_activity",
				DedupWindow: 30 * time.Minute,
			})
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "EMISSION_EXHAUSTED"})
			return
		}
		reward = adjustedReward

		granted, _, err := GrantCoinsWithCap(db, player.PlayerID, reward, time.Now().UTC(), FaucetActivity, &account.AccountID)
		if err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if err := RecordFaucetClaim(db, player.PlayerID, FaucetActivity); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		player.Coins += int64(granted)
		emitServerTelemetry(db, &account.AccountID, playerID, "faucet_claim", map[string]interface{}{
			"faucet":        FaucetActivity,
			"granted":       granted,
			"attempted":     reward,
			"playerCoins":   player.Coins,
			"remainingCap":  remainingCap,
			"availableCoins": economy.AvailableCoins(),
		})

		json.NewEncoder(w).Encode(FaucetClaimResponse{
			OK:          true,
			Reward:      granted,
			PlayerCoins: int(player.Coins),
		})
	}
}
