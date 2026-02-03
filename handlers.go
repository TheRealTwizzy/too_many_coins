package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.ServeFile(w, r, "./public"+r.URL.Path)
		return
	}

	data, err := os.ReadFile("./public/index.html")
	if err != nil {
		http.Error(w, "Failed to load index.html", 500)
		return
	}

	devMode := os.Getenv("DEV_MODE") == "true"

	injection := `<script>window.__DEV_MODE__ = ` +
		func() string {
			if devMode {
				return "true"
			}
			return "false"
		}() +
		`;</script>`

	html := strings.Replace(
		string(data),
		"<head>",
		"<head>\n"+injection,
		1,
	)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
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

		seasons := []struct {
			id        string
			startTime time.Time
			coins     int64
		}{
			{currentSeasonID(), seasonStart(), economy.CoinsInCirculation()},
			{"season-2", now.Add(-14 * 24 * time.Hour), 12000},
			{"season-3", now.Add(-7 * 24 * time.Hour), 6000},
			{"season-4", now, 1000},
		}

		const seasonLength = 28 * 24 * time.Hour

		var response []SeasonView
		var recommended string
		var maxRemaining int64 = -1

		for _, s := range seasons {
			remaining := seasonLength - now.Sub(s.startTime)
			if remaining < 0 {
				continue
			}

			seconds := int64(remaining.Seconds())
			if seconds > maxRemaining {
				maxRemaining = seconds
				recommended = s.id
			}

			emission := economy.EffectiveEmissionPerMinute(seconds, s.coins)
			response = append(response, SeasonView{
				SeasonID:              s.id,
				SecondsRemaining:      seconds,
				CoinsInCirculation:    s.coins,
				CoinEmissionPerMinute: emission,
				CurrentStarPrice:      ComputeStarPrice(s.coins, seconds),
			})
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"recommendedSeasonId": recommended,
			"seasons":             response,
		})
	}
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
		maxQty := bulkStarMaxQty()
		if quantity < 1 || quantity > maxQty {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INVALID_QUANTITY"})
			return
		}

		playerID := account.PlayerID
		if !isValidPlayerID(playerID) {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INVALID_PLAYER_ID"})
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

		err = tx.QueryRowContext(r.Context(), `
			SELECT coins, stars, burned_coins
			FROM players
			WHERE player_id = $1
			FOR UPDATE
		`, playerID).Scan(&coinsBefore, &starsBefore, new(int64))

		if err != nil {
			json.NewEncoder(w).Encode(BuyStarResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		if coinsBefore < quote.TotalCoinsSpent {
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

		var req BuyStarRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		quantity := req.Quantity
		if quantity <= 0 {
			quantity = 1
		}
		maxQty := bulkStarMaxQty()
		if quantity < 1 || quantity > maxQty {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "INVALID_QUANTITY"})
			return
		}

		playerID := account.PlayerID
		if !isValidPlayerID(playerID) {
			json.NewEncoder(w).Encode(BuyStarQuoteResponse{OK: false, Error: "INVALID_PLAYER_ID"})
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
		finalPrice := int(float64(dampenedPrice)*multiplier + 0.9999)
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
	const fallback = 0.08
	value := strings.TrimSpace(os.Getenv("BULK_STAR_GAMMA"))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
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

		price := int(float64(dampenedPrice)*variantMultiplier + 0.9999)
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

		if player.Coins < price {
			json.NewEncoder(w).Encode(BuyBoostResponse{OK: false, Error: "NOT_ENOUGH_COINS"})
			return
		}

		player.Coins -= price
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

func whitelistRequestHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		var req WhitelistRequestPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		ip := getClientIP(r)
		if ip == "" {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_IP"})
			return
		}
		pending, err := hasPendingWhitelistRequest(db, ip)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if pending {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "REQUEST_ALREADY_PENDING"})
			return
		}
		if err := createIPWhitelistRequest(db, ip, account.AccountID, req.Reason); err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		_ = createNotification(db, "admin", "", "Whitelist request pending for IP: "+ip, "warn", "#/admin", nil)
		_ = createNotification(db, "user", account.AccountID, "Whitelist request submitted. An admin will review it.", "info", "#/home", nil)
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
		rows, err := db.Query(`
			SELECT n.id, n.message, n.level, n.link, n.created_at, n.expires_at,
				(r.notification_id IS NOT NULL) AS is_read
			FROM notifications n
			LEFT JOIN notification_reads r
				ON r.notification_id = n.id AND r.account_id = $1
			WHERE (n.expires_at IS NULL OR n.expires_at > NOW())
				AND (n.account_id IS NULL OR n.account_id = $1)
				AND (
					n.target_role = 'all'
					OR n.target_role = $2
					OR (n.target_role = 'user' AND $2 = 'user')
				)
			ORDER BY n.created_at DESC
			LIMIT 60
		`, account.AccountID, account.Role)
		if err != nil {
			json.NewEncoder(w).Encode(NotificationsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer rows.Close()
		list := []NotificationItem{}
		for rows.Next() {
			var item NotificationItem
			var expires sql.NullTime
			if err := rows.Scan(&item.ID, &item.Message, &item.Level, &item.Link, &item.CreatedAt, &expires, &item.IsRead); err != nil {
				continue
			}
			if expires.Valid {
				item.ExpiresAt = &expires.Time
			}
			list = append(list, item)
		}
		json.NewEncoder(w).Encode(NotificationsResponse{OK: true, Notifications: list})
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
		for _, id := range req.IDs {
			_, _ = db.Exec(`
				INSERT INTO notification_reads (notification_id, account_id, read_at)
				VALUES ($1, $2, NOW())
				ON CONFLICT (notification_id, account_id) DO NOTHING
			`, id, account.AccountID)
		}
		json.NewEncoder(w).Encode(SimpleResponse{OK: true})
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
		allowed, err := CanSignupFromIP(db, ip)
		if err != nil {
			log.Println("signup: CanSignupFromIP error:", err)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !allowed {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "IP_LIMIT"})
			return
		}
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
			OK:           true,
			Username:     account.Username,
			DisplayName:  account.DisplayName,
			PlayerID:     account.PlayerID,
			Role:         account.Role,
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    int64(time.Until(accessExpires).Seconds()),
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
			OK:           true,
			Username:     account.Username,
			DisplayName:  account.DisplayName,
			PlayerID:     account.PlayerID,
			IsAdmin:      account.Role == "admin",
			IsModerator:  account.Role == "moderator",
			Role:         account.Role,
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    int64(time.Until(accessExpires).Seconds()),
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
		json.NewEncoder(w).Encode(AuthResponse{
			OK:          true,
			Username:    account.Username,
			DisplayName: account.DisplayName,
			PlayerID:    account.PlayerID,
			IsAdmin:     account.Role == "admin",
			IsModerator: account.Role == "moderator",
			Role:        account.Role,
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
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "SEASON_ENDED"})
			return
		}
		if !featureFlags.FaucetsEnabled {
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

		allowed, err := IsPlayerAllowedByIP(db, playerID)
		if err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !allowed {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "FAUCET_BLOCKED"})
			return
		}

		const reward = 20
		const cooldown = 24 * time.Hour

		canClaim, remaining, err := CanClaimFaucet(db, playerID, FaucetDaily, cooldown)
		if err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !canClaim {
			json.NewEncoder(w).Encode(FaucetClaimResponse{
				OK:                     false,
				Error:                  "COOLDOWN",
				NextAvailableInSeconds: int64(remaining.Seconds()),
			})
			return
		}

		if !TryDistributeCoinsWithPriority(FaucetDaily, reward) {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "EMISSION_EXHAUSTED"})
			return
		}

		player.Coins += int64(reward)
		if err := UpdatePlayerBalances(db, player.PlayerID, player.Coins, player.Stars); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if err := RecordFaucetClaim(db, player.PlayerID, FaucetDaily); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		json.NewEncoder(w).Encode(FaucetClaimResponse{
			OK:          true,
			Reward:      reward,
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
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "SEASON_ENDED"})
			return
		}
		if !featureFlags.FaucetsEnabled {
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

		allowed, err := IsPlayerAllowedByIP(db, playerID)
		if err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !allowed {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "FAUCET_BLOCKED"})
			return
		}

		reward := 3
		const cooldown = 5 * time.Minute

		if active, _, err := HasActiveBoost(db, playerID, BoostActivity); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		} else if active {
			reward += 1
		}

		canClaim, remaining, err := CanClaimFaucet(db, playerID, FaucetActivity, cooldown)
		if err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !canClaim {
			json.NewEncoder(w).Encode(FaucetClaimResponse{
				OK:                     false,
				Error:                  "COOLDOWN",
				NextAvailableInSeconds: int64(remaining.Seconds()),
			})
			return
		}

		if !TryDistributeCoinsWithPriority(FaucetActivity, reward) {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "EMISSION_EXHAUSTED"})
			return
		}

		player.Coins += int64(reward)
		if err := UpdatePlayerBalances(db, player.PlayerID, player.Coins, player.Stars); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if err := RecordFaucetClaim(db, player.PlayerID, FaucetActivity); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		json.NewEncoder(w).Encode(FaucetClaimResponse{
			OK:          true,
			Reward:      reward,
			PlayerCoins: int(player.Coins),
		})
	}
}
