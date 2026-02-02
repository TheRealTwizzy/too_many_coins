package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

/* ======================
   Request / Response Types
   ====================== */

type BuyStarRequest struct {
	SeasonID string `json:"seasonId"`
	PlayerID string `json:"playerId"`
}

type BuyStarResponse struct {
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
	StarPricePaid int    `json:"starPricePaid,omitempty"`
	PlayerCoins   int    `json:"playerCoins,omitempty"`
	PlayerStars   int    `json:"playerStars,omitempty"`
}

/* ======================
   main()
   ====================== */

func main() {
	// Environment
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}
	log.Println("App environment:", env)

	devMode := os.Getenv("DEV_MODE") == "true"
	if devMode {
		log.Println("⚠️  DEV MODE ENABLED")
	}

	// Database
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

	if err := db.Ping(); err != nil {
		log.Fatal("failed to ping database:", err)
	}
	log.Println("Connected to PostgreSQL")

	// Schema (local only)
	if env == "local" {
		if err := ensureSchema(db); err != nil {
			log.Fatal("Failed to ensure schema:", err)
		}
	}

	// Economy
	if err := economy.load("season-1", db); err != nil {
		log.Fatal("Failed to load economy state:", err)
	}

	startTickLoop(db)

	// Passive drip
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			runPassiveDrip(db)
		}
	}()

	// HTTP server
	mux := http.NewServeMux()
	registerRoutes(mux, db, devMode)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Server starting on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

/* ======================
   Routes
   ====================== */

func registerRoutes(mux *http.ServeMux, db *sql.DB, devMode bool) {
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/player", playerHandler(db))
	mux.HandleFunc("/seasons", seasonsHandler(db))
	mux.HandleFunc("/buy-star", buyStarHandler(db))
}

/* ======================
   Background Workers
   ====================== */

func runPassiveDrip(db *sql.DB) {
	now := time.Now().UTC()

	rows, err := db.Query(`
		SELECT player_id, last_active_at
		FROM players
	`)
	if err != nil {
		log.Println("drip query failed:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var playerID string
		var last time.Time

		if err := rows.Scan(&playerID, &last); err != nil {
			continue
		}

		if !CanDrip(last, now) {
			continue
		}

		if !economy.TryDistributeCoins(1) {
			return
		}

		_, err := db.Exec(`
			UPDATE players
			SET coins = coins + 1,
			    last_active_at = $2
			WHERE player_id = $1
		`, playerID, now)

		if err != nil {
			log.Println("drip update failed:", err)
		}
	}
}
