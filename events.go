package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type liveSeasonSnapshot struct {
	SeasonID                string   `json:"seasonId"`
	Status                  string   `json:"status"`
	SeasonStatus            string   `json:"season_status"`
	SeasonStartTime         string   `json:"seasonStartTime"`
	SeasonEndTime           string   `json:"seasonEndTime"`
	DayIndex                int      `json:"dayIndex"`
	TotalDays               int      `json:"totalDays"`
	SecondsRemaining        int64    `json:"secondsRemaining"`
	CoinsInCirculation      *int64   `json:"coinsInCirculation,omitempty"`
	CoinEmissionPerMinute   *float64 `json:"coinEmissionPerMinute,omitempty"`
	CurrentStarPrice        *int     `json:"currentStarPrice,omitempty"`
	NextEmissionInSeconds   *int64   `json:"nextEmissionInSeconds,omitempty"`
	MarketPressure          *float64 `json:"marketPressure,omitempty"`
	FinalStarPrice          *int     `json:"finalStarPrice,omitempty"`
	FinalCoinsInCirculation *int64   `json:"finalCoinsInCirculation,omitempty"`
	EndedAt                 *string  `json:"endedAt,omitempty"`
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
	ended := isSeasonEnded(now)
	remaining := seasonSecondsRemaining(now)
	coins := economy.CoinsInCirculation()
	activeCoins := economy.ActiveCoinsInCirculation()
	status := "active"
	if ended {
		status = "ended"
	}
	startTime := seasonStart().UTC()
	endTime := seasonEnd().UTC()
	totalDays := int(seasonLength().Hours() / 24)
	if totalDays < 1 {
		totalDays = 1
	}
	dayIndex := seasonDayIndex(now) + 1
	if dayIndex < 1 {
		dayIndex = 1
	}
	if dayIndex > totalDays {
		dayIndex = totalDays
	}
	if ended {
		remaining = 0
		dayIndex = totalDays
	}
	var emission *float64
	var marketPressure *float64
	var nextEmission *int64
	var currentPrice *int
	var liveCoins *int64
	var finalPrice *int
	var finalCoins *int64
	var endedAt *string
	if !ended {
		value := economy.EffectiveEmissionPerMinute(remaining, activeCoins)
		emission = &value
		pressure := economy.MarketPressure()
		marketPressure = &pressure
		next := nextEmissionSeconds(now)
		nextEmission = &next
		price := ComputeStarPrice(coins, remaining)
		currentPrice = &price
		liveCoins = &coins
	} else {
		var snapshotEnded time.Time
		var snapshotCoins int64
		var snapshotStars int64
		var snapshotDistributed int64
		err := db.QueryRow(`
			SELECT ended_at, coins_in_circulation, stars_purchased, coins_distributed
			FROM season_end_snapshots
			WHERE season_id = $1
		`, currentSeasonID()).Scan(&snapshotEnded, &snapshotCoins, &snapshotStars, &snapshotDistributed)
		if err == sql.ErrNoRows {
			liveCoinsValue, liveStarsValue, _ := economy.Snapshot()
			snapshotEnded = now
			snapshotCoins = int64(liveCoinsValue)
			snapshotStars = int64(liveStarsValue)
		} else if err != nil {
			log.Println("season snapshot query failed:", err)
		} else {
			_ = snapshotDistributed
		}
		params := economy.Calibration()
		pressure := economy.MarketPressure()
		final := ComputeStarPriceRawWithActive(params, int(snapshotStars), snapshotCoins, activeCoins, economy.ActivePlayers(), 0, pressure)
		finalPrice = &final
		finalCoins = &snapshotCoins
		endedValue := snapshotEnded.UTC().Format(time.RFC3339)
		endedAt = &endedValue
	}

	snapshot := liveSnapshot{
		ServerTime: now.Format(time.RFC3339),
		Season: liveSeasonSnapshot{
			SeasonID:                currentSeasonID(),
			Status:                  status,
			SeasonStatus:            status,
			SeasonStartTime:         startTime.Format(time.RFC3339),
			SeasonEndTime:           endTime.Format(time.RFC3339),
			DayIndex:                dayIndex,
			TotalDays:               totalDays,
			SecondsRemaining:        remaining,
			CoinsInCirculation:      liveCoins,
			CoinEmissionPerMinute:   emission,
			CurrentStarPrice:        currentPrice,
			NextEmissionInSeconds:   nextEmission,
			MarketPressure:          marketPressure,
			FinalStarPrice:          finalPrice,
			FinalCoinsInCirculation: finalCoins,
			EndedAt:                 endedAt,
		},
	}

	if account, _, err := getSessionAccount(db, r); err == nil && account != nil {
		snapshot.Authenticated = true
		if snapshot.Season.CurrentStarPrice != nil {
			price := computePlayerStarPrice(db, account.PlayerID, coins, remaining)
			snapshot.Season.CurrentStarPrice = &price
		}
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
