package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type BotConfig struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	Strategy       string `json:"strategy"`
	Threshold      int    `json:"threshold,omitempty"`
	MaxStarsPerDay int    `json:"maxStarsPerDay,omitempty"`
}

type BotState struct {
	Config        BotConfig
	AccessToken   string
	RefreshToken  string
	AccessExpiry  time.Time
	ActionsTaken  int
	ActionsToday  int
	LastActionDay int
}

type AuthResponse struct {
	OK           bool   `json:"ok"`
	Error        string `json:"error,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int64  `json:"expiresIn,omitempty"`
}

type RefreshResponse struct {
	OK           bool   `json:"ok"`
	Error        string `json:"error,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int64  `json:"expiresIn,omitempty"`
}

type SeasonsResponse struct {
	RecommendedSeasonID string `json:"recommendedSeasonId"`
	Seasons             []struct {
		SeasonID              string  `json:"seasonId"`
		SecondsRemaining      int64   `json:"secondsRemaining"`
		CoinsInCirculation    int64   `json:"coinsInCirculation"`
		CoinEmissionPerMinute float64 `json:"coinEmissionPerMinute"`
		CurrentStarPrice      int     `json:"currentStarPrice"`
	} `json:"seasons"`
}

type PlayerResponse struct {
	PlayerCoins int64 `json:"playerCoins"`
	PlayerStars int64 `json:"playerStars"`
}

type BuyStarResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	PlayerCoins int64  `json:"playerCoins,omitempty"`
	PlayerStars int64  `json:"playerStars,omitempty"`
}

func main() {
	if !botsEnabled() {
		logInfo("bots disabled")
		return
	}

	baseURL := strings.TrimSpace(os.Getenv("API_BASE_URL"))
	if baseURL == "" {
		logError("API_BASE_URL is required")
		os.Exit(1)
	}

	bots, err := loadBots()
	if err != nil {
		logError(fmt.Sprintf("failed to load bots: %v", err))
		os.Exit(1)
	}
	if len(bots) == 0 {
		logInfo("no bots configured")
		return
	}

	minDelay := parseEnvInt("BOT_RATE_LIMIT_MIN_MS", 3000)
	maxDelay := parseEnvInt("BOT_RATE_LIMIT_MAX_MS", 12000)
	actionProbability := parseEnvFloat("BOT_ACTION_PROBABILITY", 1.0)
	maxActions := parseEnvInt("BOT_MAX_ACTIONS_PER_RUN", 1)

	states := make([]*BotState, 0, len(bots))
	for _, bot := range bots {
		states = append(states, &BotState{Config: bot})
	}

	rand.Seed(time.Now().UnixNano())
	shuffle(states)

	client := &http.Client{Timeout: 15 * time.Second}

	for _, bot := range states {
		if bot.ActionsTaken >= maxActions {
			continue
		}
		if !canActToday(bot) {
			continue
		}
		if rand.Float64() > actionProbability {
			continue
		}

		if err := ensureAuth(client, baseURL, bot); err != nil {
			logError(fmt.Sprintf("auth failed for %s: %v", bot.Config.Username, err))
			continue
		}

		coins, _, err := fetchPlayerState(client, baseURL, bot)
		if err != nil {
			logError(fmt.Sprintf("player fetch failed for %s: %v", bot.Config.Username, err))
			continue
		}

		season, err := fetchSeasonState(client, baseURL)
		if err != nil {
			logError(fmt.Sprintf("season fetch failed for %s: %v", bot.Config.Username, err))
			continue
		}

		action := decideAction(bot, season, coins)
		if action == "buy_star" {
			if err := buyStar(client, baseURL, bot); err != nil {
				logError(fmt.Sprintf("buy star failed for %s: %v", bot.Config.Username, err))
			} else {
				logInfo(fmt.Sprintf("%s bought star", bot.Config.Username))
				bot.ActionsTaken++
				markActed(bot)
			}
		} else {
			logInfo(fmt.Sprintf("%s noop", bot.Config.Username))
		}

		sleepJitter(minDelay, maxDelay)
	}
}

func botsEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("BOTS_ENABLED")))
	if value == "" {
		return true
	}
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func loadBots() ([]BotConfig, error) {
	if raw := strings.TrimSpace(os.Getenv("BOT_LIST")); raw != "" {
		var bots []BotConfig
		if err := json.Unmarshal([]byte(raw), &bots); err != nil {
			return nil, err
		}
		return bots, nil
	}
	if raw := strings.TrimSpace(os.Getenv("BOT_LIST_PATH")); raw != "" {
		path := filepath.Clean(raw)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var bots []BotConfig
		if err := json.Unmarshal(data, &bots); err != nil {
			return nil, err
		}
		return bots, nil
	}
	return nil, nil
}

func ensureAuth(client *http.Client, baseURL string, bot *BotState) error {
	if bot.AccessToken != "" && time.Until(bot.AccessExpiry) > 2*time.Minute {
		return nil
	}
	if bot.RefreshToken != "" {
		if err := refreshAccessToken(client, baseURL, bot); err == nil {
			return nil
		}
	}
	return login(client, baseURL, bot)
}

func login(client *http.Client, baseURL string, bot *BotState) error {
	payload := map[string]string{
		"username": bot.Config.Username,
		"password": bot.Config.Password,
	}
	body, _ := json.Marshal(payload)
	res, err := client.Post(baseURL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var response AuthResponse
	if err := decodeJSON(res.Body, &response); err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}

	applyTokens(bot, response.AccessToken, response.RefreshToken, response.ExpiresIn)
	return nil
}

func refreshAccessToken(client *http.Client, baseURL string, bot *BotState) error {
	payload := map[string]string{"refreshToken": bot.RefreshToken}
	body, _ := json.Marshal(payload)
	res, err := client.Post(baseURL+"/auth/refresh", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var response RefreshResponse
	if err := decodeJSON(res.Body, &response); err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	applyTokens(bot, response.AccessToken, response.RefreshToken, response.ExpiresIn)
	return nil
}

func applyTokens(bot *BotState, accessToken string, refreshToken string, expiresIn int64) {
	bot.AccessToken = accessToken
	bot.RefreshToken = refreshToken
	if expiresIn <= 0 {
		expiresIn = int64((30 * time.Minute).Seconds())
	}
	bot.AccessExpiry = time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
}

func fetchSeasonState(client *http.Client, baseURL string) (*SeasonsResponse, error) {
	res, err := client.Get(baseURL + "/seasons")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var response SeasonsResponse
	if err := decodeJSON(res.Body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func fetchPlayerState(client *http.Client, baseURL string, bot *BotState) (int64, int64, error) {
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/player", nil)
	req.Header.Set("Authorization", "Bearer "+bot.AccessToken)
	res, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer res.Body.Close()

	var response PlayerResponse
	if err := decodeJSON(res.Body, &response); err != nil {
		return 0, 0, err
	}
	return response.PlayerCoins, response.PlayerStars, nil
}

func buyStar(client *http.Client, baseURL string, bot *BotState) error {
	payload := map[string]string{"seasonId": "season-1"}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/buy-star", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+bot.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var response BuyStarResponse
	if err := decodeJSON(res.Body, &response); err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	return nil
}

func decideAction(bot *BotState, seasons *SeasonsResponse, coins int64) string {
	if seasons == nil || len(seasons.Seasons) == 0 {
		return "noop"
	}
	season := seasons.Seasons[0]
	price := season.CurrentStarPrice
	if price <= 0 {
		return "noop"
	}

	progress := 1 - (float64(season.SecondsRemaining) / float64(28*24*3600))
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	threshold := bot.Config.Threshold
	switch bot.Config.Strategy {
	case "cautious_buyer":
		if float64(price) <= float64(coins)*0.5 {
			return "buy_star"
		}
		return "noop"
	case "late_fomo":
		if threshold == 0 {
			threshold = 200
		}
		threshold = int(float64(threshold) * (1 + (progress * 0.75)))
	case "threshold_buyer":
		if threshold == 0 {
			threshold = 200
		}
	}

	if int64(price) <= coins && price <= threshold {
		return "buy_star"
	}
	return "noop"
}

func canActToday(bot *BotState) bool {
	if bot.Config.MaxStarsPerDay <= 0 {
		return true
	}
	currentDay := time.Now().UTC().YearDay()
	if bot.LastActionDay != currentDay {
		bot.LastActionDay = currentDay
		bot.ActionsToday = 0
	}
	return bot.ActionsToday < bot.Config.MaxStarsPerDay
}

func markActed(bot *BotState) {
	currentDay := time.Now().UTC().YearDay()
	if bot.LastActionDay != currentDay {
		bot.LastActionDay = currentDay
		bot.ActionsToday = 0
	}
	bot.ActionsToday++
}

func decodeJSON(reader io.Reader, target interface{}) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func sleepJitter(minMs int, maxMs int) {
	if minMs <= 0 {
		return
	}
	if maxMs < minMs {
		maxMs = minMs
	}
	jitter := rand.Intn(maxMs-minMs+1) + minMs
	time.Sleep(time.Duration(jitter) * time.Millisecond)
}

func shuffle(states []*BotState) {
	rand.Shuffle(len(states), func(i, j int) {
		states[i], states[j] = states[j], states[i]
	})
}

func parseEnvInt(key string, fallback int) int {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			return parsed
		}
	}
	return fallback
}

func parseEnvFloat(key string, fallback float64) float64 {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func logInfo(message string) {
	fmt.Printf("[INFO] %s %s\n", time.Now().Format(time.RFC3339), message)
}

func logError(message string) {
	fmt.Printf("[ERROR] %s %s\n", time.Now().Format(time.RFC3339), message)
}
