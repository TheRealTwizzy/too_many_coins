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

type FaucetClaimRequest struct {
	PlayerID string `json:"playerId"`
	Wager    int    `json:"wager,omitempty"`
}

type FaucetClaimResponse struct {
	OK                     bool   `json:"ok"`
	Error                  string `json:"error,omitempty"`
	Reward                 int    `json:"reward,omitempty"`
	PlayerCoins            int    `json:"playerCoins,omitempty"`
	NextAvailableInSeconds int64  `json:"nextAvailableInSeconds,omitempty"`
}

type BuyVariantStarRequest struct {
	PlayerID string `json:"playerId"`
	Variant  string `json:"variant"`
}
type BuyVariantStarResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	Variant     string `json:"variant,omitempty"`
	PricePaid   int    `json:"pricePaid,omitempty"`
	PlayerCoins int    `json:"playerCoins,omitempty"`
}

type BuyBoostRequest struct {
	PlayerID  string `json:"playerId"`
	BoostType string `json:"boostType"`
}

type BuyBoostResponse struct {
	OK          bool      `json:"ok"`
	Error       string    `json:"error,omitempty"`
	BoostType   string    `json:"boostType,omitempty"`
	ExpiresAt   time.Time `json:"expiresAt,omitempty"`
	PlayerCoins int       `json:"playerCoins,omitempty"`
}

type BurnCoinsRequest struct {
	PlayerID string `json:"playerId"`
	Amount   int    `json:"amount"`
}

type BurnCoinsResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	Amount      int    `json:"amount,omitempty"`
	PlayerCoins int    `json:"playerCoins,omitempty"`
	BurnedTotal int    `json:"burnedTotal,omitempty"`
}

type SignupRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	OK           bool   `json:"ok"`
	Error        string `json:"error,omitempty"`
	Username     string `json:"username,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	PlayerID     string `json:"playerId,omitempty"`
	IsAdmin      bool   `json:"isAdmin,omitempty"`
	IsModerator  bool   `json:"isModerator,omitempty"`
	Role         string `json:"role,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int64  `json:"expiresIn,omitempty"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type RefreshTokenResponse struct {
	OK           bool   `json:"ok"`
	Error        string `json:"error,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int64  `json:"expiresIn,omitempty"`
}

type LeaderboardResponse struct {
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Total    int                `json:"total"`
	Results  []LeaderboardEntry `json:"results"`
}

type LeaderboardEntry struct {
	Rank               int    `json:"rank"`
	PlayerID           string `json:"playerId"`
	DisplayName        string `json:"displayName"`
	Stars              int64  `json:"stars"`
	CoinsSpentLifetime int64  `json:"coinsSpentLifetime"`
	LastStarAcquiredAt string `json:"lastStarAcquiredAt,omitempty"`
	IsBot              bool   `json:"isBot"`
	BotProfile         string `json:"botProfile,omitempty"`
}

type ProfileUpdateRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
	Bio         string `json:"bio,omitempty"`
	Pronouns    string `json:"pronouns,omitempty"`
	Location    string `json:"location,omitempty"`
	Website     string `json:"website,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

type ProfileResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
	Bio         string `json:"bio,omitempty"`
	Pronouns    string `json:"pronouns,omitempty"`
	Location    string `json:"location,omitempty"`
	Website     string `json:"website,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

type PasswordResetRequest struct {
	Identifier string `json:"identifier"`
}

type PasswordResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type SimpleResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type WhitelistRequestPayload struct {
	Reason string `json:"reason,omitempty"`
}

type AdminWhitelistRequestListResponse struct {
	OK       bool                   `json:"ok"`
	Error    string                 `json:"error,omitempty"`
	Requests []WhitelistRequestView `json:"requests,omitempty"`
}

type WhitelistRequestView struct {
	RequestID string    `json:"requestId"`
	IP        string    `json:"ip"`
	AccountID string    `json:"accountId,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type AdminWhitelistResolveRequest struct {
	RequestID   string `json:"requestId"`
	Decision    string `json:"decision"`
	MaxAccounts int    `json:"maxAccounts,omitempty"`
}

type NotificationItem struct {
	ID        int64      `json:"id"`
	Message   string     `json:"message"`
	Level     string     `json:"level"`
	Link      string     `json:"link,omitempty"`
	IsRead    bool       `json:"isRead"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

type NotificationsResponse struct {
	OK            bool               `json:"ok"`
	Error         string             `json:"error,omitempty"`
	Notifications []NotificationItem `json:"notifications,omitempty"`
}

type NotificationAckRequest struct {
	IDs []int64 `json:"ids"`
}

type AdminNotificationCreateRequest struct {
	TargetRole string `json:"targetRole"`
	AccountID  string `json:"accountId,omitempty"`
	Message    string `json:"message"`
	Link       string `json:"link,omitempty"`
	Level      string `json:"level,omitempty"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
}

type AdminPlayerControlRequest struct {
	PlayerID       string   `json:"playerId,omitempty"`
	Username       string   `json:"username,omitempty"`
	SetCoins       *int64   `json:"setCoins,omitempty"`
	AddCoins       *int64   `json:"addCoins,omitempty"`
	SetStars       *int64   `json:"setStars,omitempty"`
	AddStars       *int64   `json:"addStars,omitempty"`
	DripMultiplier *float64 `json:"dripMultiplier,omitempty"`
	DripPaused     *bool    `json:"dripPaused,omitempty"`
	IsBot          *bool    `json:"isBot,omitempty"`
	BotProfile     *string  `json:"botProfile,omitempty"`
	TouchActive    bool     `json:"touchActive,omitempty"`
}

type AdminPlayerControlResponse struct {
	OK             bool      `json:"ok"`
	Error          string    `json:"error,omitempty"`
	PlayerID       string    `json:"playerId,omitempty"`
	Coins          int64     `json:"coins,omitempty"`
	Stars          int64     `json:"stars,omitempty"`
	DripMultiplier float64   `json:"dripMultiplier,omitempty"`
	DripPaused     bool      `json:"dripPaused,omitempty"`
	IsBot          bool      `json:"isBot,omitempty"`
	BotProfile     string    `json:"botProfile,omitempty"`
	LastActiveAt   time.Time `json:"lastActiveAt,omitempty"`
	LastGrantAt    time.Time `json:"lastGrantAt,omitempty"`
}

type AdminGlobalSettingsRequest struct {
	ActiveDripIntervalSeconds *int  `json:"activeDripIntervalSeconds,omitempty"`
	IdleDripIntervalSeconds   *int  `json:"idleDripIntervalSeconds,omitempty"`
	ActiveDripAmount          *int  `json:"activeDripAmount,omitempty"`
	IdleDripAmount            *int  `json:"idleDripAmount,omitempty"`
	ActivityWindowSeconds     *int  `json:"activityWindowSeconds,omitempty"`
	DripEnabled               *bool `json:"dripEnabled,omitempty"`
}

type AdminGlobalSettingsResponse struct {
	OK       bool           `json:"ok"`
	Error    string         `json:"error,omitempty"`
	Settings GlobalSettings `json:"settings,omitempty"`
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

	// Schema (all environments)
	if err := ensureSchema(db); err != nil {
		log.Fatal("Failed to ensure schema:", err)
	}

	// Economy
	if err := economy.load(currentSeasonID(), db); err != nil {
		log.Fatal("Failed to load economy state:", err)
	}
	if err := LoadGlobalSettings(db); err != nil {
		log.Println("Failed to load global settings:", err)
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
	mux.HandleFunc("/events", eventsHandler(db))
	mux.HandleFunc("/buy-star", buyStarHandler(db))
	mux.HandleFunc("/buy-variant-star", buyVariantStarHandler(db))
	mux.HandleFunc("/buy-boost", buyBoostHandler(db))
	mux.HandleFunc("/burn-coins", burnCoinsHandler(db))
	mux.HandleFunc("/claim-daily", dailyClaimHandler(db))
	mux.HandleFunc("/claim-activity", activityClaimHandler(db))
	mux.HandleFunc("/auth/signup", signupHandler(db))
	mux.HandleFunc("/auth/login", loginHandler(db))
	mux.HandleFunc("/auth/logout", logoutHandler(db))
	mux.HandleFunc("/auth/me", meHandler(db))
	mux.HandleFunc("/auth/refresh", refreshTokenHandler(db))
	mux.HandleFunc("/auth/request-reset", requestPasswordResetHandler(db))
	mux.HandleFunc("/auth/reset-password", resetPasswordHandler(db))
	mux.HandleFunc("/auth/request-whitelist", whitelistRequestHandler(db))
	mux.HandleFunc("/notifications", notificationsHandler(db))
	mux.HandleFunc("/notifications/ack", notificationsAckHandler(db))
	mux.HandleFunc("/activity", activityHandler(db))
	mux.HandleFunc("/profile", profileHandler(db))
	mux.HandleFunc("/telemetry", telemetryHandler(db))
	mux.HandleFunc("/admin/telemetry", adminTelemetryHandler(db))
	mux.HandleFunc("/admin/economy", adminEconomyHandler(db))
	mux.HandleFunc("/admin/set-key", adminKeySetHandler(db))
	mux.HandleFunc("/admin/role", adminRoleHandler(db))
	mux.HandleFunc("/admin/ip-whitelist", adminIPWhitelistHandler(db))
	mux.HandleFunc("/admin/whitelist-requests", adminWhitelistRequestsHandler(db))
	mux.HandleFunc("/admin/notifications", adminNotificationsHandler(db))
	mux.HandleFunc("/admin/player-controls", adminPlayerControlsHandler(db))
	mux.HandleFunc("/admin/settings", adminSettingsHandler(db))
	mux.HandleFunc("/admin/star-purchases", adminStarPurchaseLogHandler(db))
	mux.HandleFunc("/moderator/profile", moderatorProfileHandler(db))
	mux.HandleFunc("/leaderboard", leaderboardHandler(db))
}

/* ======================
   Background Workers
   ====================== */

func runPassiveDrip(db *sql.DB) {
	now := time.Now().UTC()
	if isSeasonEnded(now) {
		return
	}
	settings := GetGlobalSettings()
	if !settings.DripEnabled {
		return
	}
	activeDripInterval := time.Duration(settings.ActiveDripIntervalSeconds) * time.Second
	idleDripInterval := time.Duration(settings.IdleDripIntervalSeconds) * time.Second
	activeDripAmount := settings.ActiveDripAmount
	idleDripAmount := settings.IdleDripAmount
	if activeDripInterval <= 0 {
		activeDripInterval = time.Minute
	}
	if idleDripInterval <= 0 {
		idleDripInterval = 4 * time.Minute
	}
	if activeDripAmount <= 0 {
		activeDripAmount = 2
	}
	if idleDripAmount <= 0 {
		idleDripAmount = 1
	}
	activityWindow := ActiveActivityWindow()

	rows, err := db.Query(`
		SELECT player_id, last_active_at, last_coin_grant_at, drip_multiplier, drip_paused
		FROM players
	`)
	if err != nil {
		log.Println("drip query failed:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var playerID string
		var lastActive time.Time
		var lastGrant time.Time
		var dripMultiplier float64
		var dripPaused bool

		if err := rows.Scan(&playerID, &lastActive, &lastGrant, &dripMultiplier, &dripPaused); err != nil {
			continue
		}
		if dripPaused {
			continue
		}

		inactiveFor := now.Sub(lastActive)
		dripInterval := idleDripInterval
		dripAmount := idleDripAmount
		if inactiveFor <= activityWindow {
			dripInterval = activeDripInterval
			dripAmount = activeDripAmount
		}

		if now.Sub(lastGrant) < dripInterval {
			continue
		}

		allowed, err := IsPlayerAllowedByIP(db, playerID)
		if err != nil {
			log.Println("drip ip check failed:", err)
			continue
		}
		if !allowed {
			continue
		}

		adjusted := int(float64(dripAmount) * dripMultiplier)
		if adjusted < 1 {
			adjusted = 1
		}
		if !economy.TryDistributeCoins(adjusted) {
			return
		}

		_, err = db.Exec(`
			UPDATE players
			SET coins = coins + $3,
			    last_coin_grant_at = $2
			WHERE player_id = $1
		`, playerID, now, adjusted)

		if err != nil {
			log.Println("drip update failed:", err)
		}
	}
}
