package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

type liveSeasonSnapshot struct {
	SeasonID              string  `json:"seasonId"`
	SecondsRemaining      int64   `json:"secondsRemaining"`
	CoinsInCirculation    int64   `json:"coinsInCirculation"`
	CoinEmissionPerMinute float64 `json:"coinEmissionPerMinute"`
	CurrentStarPrice      int     `json:"currentStarPrice"`
	NextEmissionInSeconds int64   `json:"nextEmissionInSeconds"`
}

type liveSnapshot struct {
	ServerTime    string             `json:"serverTime"`
	Authenticated bool               `json:"authenticated"`
	Season        liveSeasonSnapshot `json:"season"`
	PlayerCoins   int64              `json:"playerCoins,omitempty"`
	PlayerStars   int64              `json:"playerStars,omitempty"`
}

func buildLiveSnapshot(db *sql.DB, r *http.Request) liveSnapshot {
	now := time.Now().UTC()
	remaining := seasonSecondsRemaining(now)
	coins := economy.CoinsInCirculation()

	snapshot := liveSnapshot{
		ServerTime: now.Format(time.RFC3339),
		Season: liveSeasonSnapshot{
			SeasonID:              currentSeasonID(),
			SecondsRemaining:      remaining,
			CoinsInCirculation:    coins,
			CoinEmissionPerMinute: economy.EffectiveEmissionPerMinute(remaining, coins),
			CurrentStarPrice:      ComputeStarPrice(coins, remaining),
			NextEmissionInSeconds: nextEmissionSeconds(now),
		},
	}

	if account, _, err := getSessionAccount(db, r); err == nil && account != nil {
		snapshot.Authenticated = true
		if player, err := LoadPlayer(db, account.PlayerID); err == nil && player != nil {
			snapshot.PlayerCoins = player.Coins
			snapshot.PlayerStars = player.Stars
		}
	}

	return snapshot
}

func eventsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		sendSnapshot := func() bool {
			payload, err := json.Marshal(buildLiveSnapshot(db, r))
			if err != nil {
				return false
			}
			if _, err := w.Write([]byte("event: snapshot\n")); err != nil {
				return false
			}
			if _, err := w.Write([]byte("data: ")); err != nil {
				return false
			}
			if _, err := w.Write(payload); err != nil {
				return false
			}
			if _, err := w.Write([]byte("\n\n")); err != nil {
				return false
			}
			flusher.Flush()
			return true
		}

		if !sendSnapshot() {
			return
		}

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !sendSnapshot() {
					return
				}
			}
		}
	}
}
