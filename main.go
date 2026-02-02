package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
	
	_ "github.com/lib/pq"
)

func main() {
	
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}
	
	log.Println("App environment:", env)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("failed to open database:", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	err = db.Ping()
	if err != nil {
		log.Fatal("failed to ping database:", err)
	}

	log.Println("Connected to PostgreSQL")
	
	if env == "local" {
	err = ensureSchema(db)
		if err != nil {
			log.Fatal("Failed to ensure local schema:", err)
		}
	}

	err = economy.load("season-1", db)
	if err != nil {
		log.Fatal("Failed to load economy state:", err)
	}
	
	startTickLoop(db)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	
	mux.HandleFunc("/seasons", func(w http.ResponseWriter, r *http.Request) {
		type SeasonView struct {
			SeasonID               string  `json:"seasonId"`
			SecondsRemaining       int64   `json:"secondsRemaining"`
			CoinsInCirculation     int64   `json:"coinsInCirculation"`
			CoinEmissionPerMinute  float64 `json:"coinEmissionPerMinute"`
		}
	
		now := time.Now().UTC()
	
		// Stub season start times (7-day stagger)
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
	
		var responseSeasons []SeasonView
		var recommendedSeasonID string
		var maxRemaining int64 = -1
	
		for _, s := range seasons {
			elapsed := now.Sub(s.startTime)
			remaining := seasonLength - elapsed
			if remaining < 0 {
				continue
			}
	
			secondsRemaining := int64(remaining.Seconds())
	
			if secondsRemaining > maxRemaining {
				maxRemaining = secondsRemaining
				recommendedSeasonID = s.id
			}
	
			responseSeasons = append(responseSeasons, SeasonView{
				SeasonID:              s.id,
				SecondsRemaining:      secondsRemaining,
				CoinsInCirculation:    s.coins,
				CoinEmissionPerMinute: economy.EmissionPerMinute(),
			})
		}
	
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"recommendedSeasonId": recommendedSeasonID,
			"seasons":             responseSeasons,
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Server starting on :" + port)
	err = http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Fatal(err)
	}
}
