package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
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

			response = append(response, SeasonView{
				SeasonID:              s.id,
				SecondsRemaining:      seconds,
				CoinsInCirculation:    s.coins,
				CoinEmissionPerMinute: economy.EmissionPerMinute(),
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

		price := ComputeStarPrice(
			economy.CoinsInCirculation(),
			28*24*3600,
		)

		dampenedPrice, err := ComputeDampenedStarPrice(db, playerID, price)
		if err != nil {
			json.NewEncoder(w).Encode(BuyStarResponse{
				OK: false, Error: "INTERNAL_ERROR",
			})
			return
		}
		price = dampenedPrice

		if player.Coins < int64(price) {
			json.NewEncoder(w).Encode(BuyStarResponse{
				OK: false, Error: "NOT_ENOUGH_COINS",
			})
			return
		}

		player.Coins -= int64(price)
		player.Stars++

		UpdatePlayerBalances(db, player.PlayerID, player.Coins, player.Stars)
		economy.IncrementStars()

		json.NewEncoder(w).Encode(BuyStarResponse{
			OK:            true,
			StarPricePaid: price,
			PlayerCoins:   int(player.Coins),
			PlayerStars:   int(player.Stars),
		})
	}
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

func auctionStatusHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := GetAuctionStatus(db)
		if err != nil {
			json.NewEncoder(w).Encode(AuctionStatusResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		json.NewEncoder(w).Encode(AuctionStatusResponse{OK: true, Status: status})
	}
}

func auctionBidHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if isSeasonEnded(time.Now().UTC()) {
			json.NewEncoder(w).Encode(AuctionBidResponse{OK: false, Error: "SEASON_ENDED"})
			return
		}
		if !featureFlags.SinksEnabled {
			json.NewEncoder(w).Encode(AuctionBidResponse{OK: false, Error: "FEATURE_DISABLED"})
			return
		}

		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		playerID := account.PlayerID
		if !isValidPlayerID(playerID) {
			json.NewEncoder(w).Encode(AuctionBidResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		var req AuctionBidRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(AuctionBidResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		if req.Bid <= 0 {
			json.NewEncoder(w).Encode(AuctionBidResponse{OK: false, Error: "INVALID_BID"})
			return
		}

		status, err := PlaceAuctionBid(db, playerID, req.Bid)
		if err != nil {
			switch {
			case errors.Is(err, errAuctionNotFound):
				json.NewEncoder(w).Encode(AuctionBidResponse{OK: false, Error: "AUCTION_NOT_FOUND"})
			case err.Error() == "BID_TOO_LOW":
				json.NewEncoder(w).Encode(AuctionBidResponse{OK: false, Error: "BID_TOO_LOW"})
			case err.Error() == "NOT_ENOUGH_COINS":
				json.NewEncoder(w).Encode(AuctionBidResponse{OK: false, Error: "NOT_ENOUGH_COINS"})
			default:
				json.NewEncoder(w).Encode(AuctionBidResponse{OK: false, Error: "INTERNAL_ERROR"})
			}
			return
		}

		json.NewEncoder(w).Encode(AuctionBidResponse{OK: true, Status: status})
	}
}

func signupHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if account, _, err := getSessionAccount(db, r); err == nil && account != nil {
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "ALREADY_LOGGED_IN"})
			return
		}

		if ip := getClientIP(r); ip != "" {
			allowed, err := CanSignupFromIP(db, ip)
			if err != nil {
				json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if !allowed {
				json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "IP_SIGNUP_BLOCKED"})
				return
			}
		}

		var req SignupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		account, err := createAccount(db, req.Username, req.Password, req.DisplayName, req.Email)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "USERNAME_TAKEN"})
				return
			}
			if err.Error() == "INVALID_USERNAME" || err.Error() == "INVALID_PASSWORD" || err.Error() == "INVALID_EMAIL" {
				json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: err.Error()})
				return
			}
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		if _, err := LoadOrCreatePlayer(db, account.PlayerID); err != nil {
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		if ip := getClientIP(r); ip != "" {
			isNew, err := RecordPlayerIP(db, account.PlayerID, ip)
			if err != nil {
				log.Println("Failed to record player IP on signup:", err)
			} else if isNew {
				if err := ApplyIPDampeningDelay(db, account.PlayerID, ip); err != nil {
					log.Println("Failed to apply IP dampening delay on signup:", err)
				}
			}
		}

		sessionID, expiresAt, err := createSession(db, account.AccountID)
		if err != nil {
			json.NewEncoder(w).Encode(AuthResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		writeSessionCookie(w, sessionID, expiresAt)
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
		_ = createNotification(db, "admin", "", "Whitelist request pending for IP: "+ip, "warn", nil)
		_ = createNotification(db, "user", account.AccountID, "Whitelist request submitted. An admin will review it.", "info", nil)
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
			SELECT n.id, n.message, n.level, n.created_at, n.expires_at
			FROM notifications n
			LEFT JOIN notification_reads r
				ON r.notification_id = n.id AND r.account_id = $1
			WHERE r.notification_id IS NULL
				AND (n.expires_at IS NULL OR n.expires_at > NOW())
				AND (n.account_id IS NULL OR n.account_id = $1)
				AND (
					n.target_role = 'all'
					OR n.target_role = $2
					OR (n.target_role = 'user' AND $2 = 'user')
				)
			ORDER BY n.created_at DESC
			LIMIT 20
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
			if err := rows.Scan(&item.ID, &item.Message, &item.Level, &item.CreatedAt, &expires); err != nil {
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
			normalizedEmail := ""
			if strings.TrimSpace(req.Email) != "" {
				var err error
				normalizedEmail, err = normalizeEmail(req.Email)
				if err != nil {
					json.NewEncoder(w).Encode(ProfileResponse{OK: false, Error: "INVALID_EMAIL"})
					return
				}
			}

			if normalizedEmail != "" {
				_, err := db.Exec(`
					UPDATE accounts
					SET display_name = $2, email = $3
					WHERE account_id = $1
				`, account.AccountID, displayName, normalizedEmail)
				if err != nil {
					json.NewEncoder(w).Encode(ProfileResponse{OK: false, Error: "INTERNAL_ERROR"})
					return
				}
				account.Email = normalizedEmail
			} else {
				_, err := db.Exec(`
					UPDATE accounts
					SET display_name = $2
					WHERE account_id = $1
				`, account.AccountID, displayName)
				if err != nil {
					json.NewEncoder(w).Encode(ProfileResponse{OK: false, Error: "INTERNAL_ERROR"})
					return
				}
			}

			json.NewEncoder(w).Encode(ProfileResponse{
				OK:          true,
				Username:    account.Username,
				DisplayName: displayName,
				Email:       account.Email,
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

func riskRollHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if isSeasonEnded(time.Now().UTC()) {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "SEASON_ENDED"})
			return
		}
		if !featureFlags.FaucetsEnabled {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "FEATURE_DISABLED"})
			return
		}

		account, ok := requireSession(db, w, r)
		if !ok {
			return
		}
		playerID := account.PlayerID
		if !isValidPlayerID(playerID) {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		var req FaucetClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		wager := req.Wager
		if wager <= 0 || wager > 5 {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INVALID_WAGER"})
			return
		}

		player, err := LoadPlayer(db, playerID)
		if err != nil || player == nil {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "PLAYER_NOT_REGISTERED"})
			return
		}

		if player.Coins < int64(wager) {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "NOT_ENOUGH_COINS"})
			return
		}

		allowed, err := IsPlayerAllowedByIP(db, playerID)
		if err != nil {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !allowed {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "FAUCET_BLOCKED"})
			return
		}

		const cooldown = time.Minute
		canClaim, remaining, err := CanClaimFaucet(db, playerID, FaucetRisk, cooldown)
		if err != nil {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !canClaim {
			json.NewEncoder(w).Encode(RiskRollResponse{
				OK:                     false,
				Error:                  "COOLDOWN",
				NextAvailableInSeconds: int64(remaining.Seconds()),
			})
			return
		}

		won, err := rollWin50()
		if err != nil {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		payout := 0
		if won {
			payout = wager * 3
			if !TryDistributeCoinsWithPriority(FaucetRisk, payout) {
				json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "EMISSION_EXHAUSTED"})
				return
			}
		}

		player.Coins -= int64(wager)
		player.Coins += int64(payout)

		if err := UpdatePlayerBalances(db, player.PlayerID, player.Coins, player.Stars); err != nil {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if err := RecordFaucetClaim(db, player.PlayerID, FaucetRisk); err != nil {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		json.NewEncoder(w).Encode(RiskRollResponse{
			OK:          true,
			Won:         won,
			Wager:       wager,
			Payout:      payout,
			PlayerCoins: int(player.Coins),
		})
	}
}
