package main

import (
	"database/sql"
	"encoding/json"
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

func playerHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID := r.URL.Query().Get("playerId")
		if !isValidPlayerID(playerID) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if ip := getClientIP(r); ip != "" {
			log.Printf("IP association: ip=%s playerId=%s", ip, playerID)
		}

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
			{"season-1", now.Add(-21 * 24 * time.Hour), economy.CoinsInCirculation()},
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

		var req BuyStarRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(BuyStarResponse{
				OK: false, Error: "INVALID_REQUEST",
			})
			return
		}

		if !isValidPlayerID(req.PlayerID) {
			json.NewEncoder(w).Encode(BuyStarResponse{
				OK: false, Error: "INVALID_PLAYER_ID",
			})
			return
		}

		if ip := getClientIP(r); ip != "" {
			log.Printf("IP association: ip=%s playerId=%s", ip, req.PlayerID)
		}

		player, err := LoadPlayer(db, req.PlayerID)
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

		dampenedPrice, err := ComputeDampenedStarPrice(db, req.PlayerID, price)
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

func dailyClaimHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req FaucetClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		if !isValidPlayerID(req.PlayerID) {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		player, err := LoadPlayer(db, req.PlayerID)
		if err != nil || player == nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "PLAYER_NOT_REGISTERED"})
			return
		}

		allowed, err := IsPlayerAllowedByIP(db, req.PlayerID)
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

		canClaim, remaining, err := CanClaimFaucet(db, req.PlayerID, FaucetDaily, cooldown)
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

		player.Coins += reward
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

		var req FaucetClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		if !isValidPlayerID(req.PlayerID) {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		player, err := LoadPlayer(db, req.PlayerID)
		if err != nil || player == nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "PLAYER_NOT_REGISTERED"})
			return
		}

		allowed, err := IsPlayerAllowedByIP(db, req.PlayerID)
		if err != nil {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !allowed {
			json.NewEncoder(w).Encode(FaucetClaimResponse{OK: false, Error: "FAUCET_BLOCKED"})
			return
		}

		const reward = 3
		const cooldown = 5 * time.Minute

		canClaim, remaining, err := CanClaimFaucet(db, req.PlayerID, FaucetActivity, cooldown)
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

		player.Coins += reward
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

		var req FaucetClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		if !isValidPlayerID(req.PlayerID) {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INVALID_PLAYER_ID"})
			return
		}

		wager := req.Wager
		if wager <= 0 || wager > 5 {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INVALID_WAGER"})
			return
		}

		player, err := LoadPlayer(db, req.PlayerID)
		if err != nil || player == nil {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "PLAYER_NOT_REGISTERED"})
			return
		}

		if player.Coins < int64(wager) {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "NOT_ENOUGH_COINS"})
			return
		}

		allowed, err := IsPlayerAllowedByIP(db, req.PlayerID)
		if err != nil {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !allowed {
			json.NewEncoder(w).Encode(RiskRollResponse{OK: false, Error: "FAUCET_BLOCKED"})
			return
		}

		const cooldown = time.Minute
		canClaim, remaining, err := CanClaimFaucet(db, req.PlayerID, FaucetRisk, cooldown)
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
