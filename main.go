package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

//go:embed public/*
var content embed.FS

var publicFS = mustSubFS(content, "public")

func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

/* ======================
   Request / Response Types
   ====================== */

type BuyStarRequest struct {
	SeasonID string `json:"seasonId"`
	PlayerID string `json:"playerId"`
	Quantity int    `json:"quantity,omitempty"`
}

type BulkStarBreakdown struct {
	Index          int     `json:"index"`
	BasePrice      int     `json:"basePrice"`
	BulkMultiplier float64 `json:"bulkMultiplier"`
	FinalPrice     int     `json:"finalPrice"`
}

type BuyStarResponse struct {
	OK              bool                `json:"ok"`
	Error           string              `json:"error,omitempty"`
	StarPricePaid   int                 `json:"starPricePaid,omitempty"`
	PlayerCoins     int                 `json:"playerCoins,omitempty"`
	PlayerStars     int                 `json:"playerStars,omitempty"`
	StarsPurchased  int                 `json:"starsPurchased,omitempty"`
	TotalCoinsSpent int                 `json:"totalCoinsSpent,omitempty"`
	FinalStarPrice  int                 `json:"finalStarPrice,omitempty"`
	Breakdown       []BulkStarBreakdown `json:"breakdown,omitempty"`
	Warning         string              `json:"warning,omitempty"`
	WarningLevel    string              `json:"warningLevel,omitempty"`
}

type BuyStarQuoteResponse struct {
	OK              bool                `json:"ok"`
	Error           string              `json:"error,omitempty"`
	StarsRequested  int                 `json:"starsRequested,omitempty"`
	TotalCoinsSpent int                 `json:"totalCoinsSpent,omitempty"`
	FinalStarPrice  int                 `json:"finalStarPrice,omitempty"`
	Breakdown       []BulkStarBreakdown `json:"breakdown,omitempty"`
	Warning         string              `json:"warning,omitempty"`
	WarningLevel    string              `json:"warningLevel,omitempty"`
	CanAfford       bool                `json:"canAfford,omitempty"`
	Shortfall       int                 `json:"shortfall,omitempty"`
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
	OK                 bool   `json:"ok"`
	Error              string `json:"error,omitempty"`
	Username           string `json:"username,omitempty"`
	DisplayName        string `json:"displayName,omitempty"`
	PlayerID           string `json:"playerId,omitempty"`
	IsAdmin            bool   `json:"isAdmin,omitempty"`
	IsModerator        bool   `json:"isModerator,omitempty"`
	Role               string `json:"role,omitempty"`
	MustChangePassword bool   `json:"mustChangePassword,omitempty"`
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

type NotificationItem struct {
	ID             int64           `json:"id"`
	Message        string          `json:"message"`
	Level          string          `json:"level"`
	Link           string          `json:"link,omitempty"`
	Category       string          `json:"category,omitempty"`
	Type           string          `json:"type,omitempty"`
	Priority       string          `json:"priority,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	AckRequired    bool            `json:"ackRequired"`
	IsAcknowledged bool            `json:"isAcknowledged"`
	AcknowledgedAt *time.Time      `json:"acknowledgedAt,omitempty"`
	IsRead         bool            `json:"isRead"`
	CreatedAt      time.Time       `json:"createdAt"`
	ExpiresAt      *time.Time      `json:"expiresAt,omitempty"`
}

type NotificationsResponse struct {
	OK            bool               `json:"ok"`
	Error         string             `json:"error,omitempty"`
	Notifications []NotificationItem `json:"notifications,omitempty"`
}

type NotificationAckRequest struct {
	IDs []int64 `json:"ids"`
}

type NotificationDeleteRequest struct {
	IDs []int64 `json:"ids"`
}

type NotificationSettingItem struct {
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"`
	PushEnabled bool   `json:"pushEnabled"`
}

type NotificationSettingsResponse struct {
	OK       bool                      `json:"ok"`
	Error    string                    `json:"error,omitempty"`
	Settings []NotificationSettingItem `json:"settings,omitempty"`
}

type NotificationSettingsUpdateRequest struct {
	Categories []NotificationSettingItem `json:"categories"`
}

type AdminNotificationCreateRequest struct {
	TargetRole         string          `json:"targetRole,omitempty"`
	RecipientRole      string          `json:"recipientRole,omitempty"`
	AccountID          string          `json:"accountId,omitempty"`
	RecipientAccountID string          `json:"recipientAccountId,omitempty"`
	SeasonID           string          `json:"seasonId,omitempty"`
	Category           string          `json:"category,omitempty"`
	Type               string          `json:"type,omitempty"`
	Priority           string          `json:"priority,omitempty"`
	Payload            json.RawMessage `json:"payload,omitempty"`
	Message            string          `json:"message"`
	Link               string          `json:"link,omitempty"`
	Level              string          `json:"level,omitempty"`
	ExpiresAt          string          `json:"expiresAt,omitempty"`
	AckRequired        bool            `json:"ackRequired,omitempty"`
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
	BotsEnabled               *bool `json:"botsEnabled,omitempty"`
	BotMinStarIntervalSeconds *int  `json:"botMinStarIntervalSeconds,omitempty"`
}

type AdminGlobalSettingsResponse struct {
	OK       bool           `json:"ok"`
	Error    string         `json:"error,omitempty"`
	Settings GlobalSettings `json:"settings,omitempty"`
}

type AdminBotListItem struct {
	PlayerID    string `json:"playerId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	IsBot       bool   `json:"isBot"`
	BotProfile  string `json:"botProfile,omitempty"`
}

type AdminBotListResponse struct {
	OK    bool               `json:"ok"`
	Error string             `json:"error,omitempty"`
	Bots  []AdminBotListItem `json:"bots,omitempty"`
}

type AdminProfileActionRequest struct {
	Username string `json:"username"`
	Action   string `json:"action"`
}

type AdminProfileActionResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Username string `json:"username,omitempty"`
	Role     string `json:"role,omitempty"`
	Frozen   bool   `json:"frozen,omitempty"`
}

type AdminBotCreateRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Password    string `json:"password,omitempty"`
	BotProfile  string `json:"botProfile,omitempty"`
}

type AdminBotCreateResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	PlayerID    string `json:"playerId,omitempty"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Password    string `json:"password,omitempty"`
}

type AdminBotDeleteRequest struct {
	PlayerID string `json:"playerId,omitempty"`
	Username string `json:"username,omitempty"`
}

type AdminBotDeleteResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	PlayerID string `json:"playerId,omitempty"`
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

	phase := CurrentPhase()
	log.Println("Phase:", phase)
	seasonDays := int(seasonLength().Hours() / 24)
	log.Println("Season length (days):", seasonDays)
	if phase == PhaseAlpha {
		log.Println("Alpha season extension days:", os.Getenv("ALPHA_SEASON_EXTENSION_DAYS"))
		log.Println("Alpha season extension reason:", os.Getenv("ALPHA_SEASON_EXTENSION_REASON"))
	}

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
	if strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) == "alpha" {
		if phase, ok := parsePhaseFromEnv("PHASE"); ok && phase != PhaseAlpha {
			log.Fatal("PHASE conflicts with APP_ENV=alpha; refusing to start")
		}
		if seasonLength() > time.Duration(alphaSeasonMaxDays)*24*time.Hour {
			log.Fatal("Alpha season length exceeds max days; refusing to start")
		}
	}

	ctx := context.Background()
	lockConn, acquired, err := acquireStartupLock(ctx, db)
	if err != nil {
		log.Fatal("Failed to acquire startup lock:", err)
	}
	if acquired {
		startupLockConn = lockConn
		log.Println("Startup lock acquired; running Alpha initialization")
		if err := ensureAlphaAdmin(ctx, db); err != nil {
			log.Fatal("Alpha admin bootstrap failed:", err)
		}
		if err := ensureActiveSeason(ctx, db); err != nil {
			log.Fatal("Failed to ensure active season:", err)
		}
	} else {
		log.Println("Startup lock held by another instance; skipping leader-only initialization")
	}
	if !acquired && lockConn != nil {
		_ = lockConn.Close()
	}

	// Economy (safe for all instances; writes are idempotent)
	if err := economy.load(currentSeasonID(), db); err != nil {
		log.Fatal("Failed to load economy state:", err)
	}
	if _, err := LoadOrCalibrateSeason(db, currentSeasonID()); err != nil {
		log.Fatal("Failed to calibrate economy:", err)
	}
	if err := LoadGlobalSettings(db); err != nil {
		log.Println("Failed to load global settings:", err)
	}
	if CurrentPhase() == PhaseAlpha {
		settings := GetGlobalSettings()
		if settings.DripEnabled {
			log.Println("WARN: passive drip enabled in settings; overriding to disabled for alpha")
			if _, err := UpdateGlobalSettings(db, map[string]string{"drip_enabled": "false"}); err != nil {
				log.Println("Failed to persist alpha drip override:", err)
				settingsMu.Lock()
				cachedSettings.DripEnabled = false
				settingsMu.Unlock()
			}
		}
		log.Println("ECONOMY_CONFIG: passive_drip=DISABLED (alpha default)")
	}

	if acquired {
		startTickLoop(db)
		startNotificationPruner(db)

		// Passive drip
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()

			for range ticker.C {
				runPassiveDrip(db)
			}
		}()
	}

	// HTTP server
	mux := http.NewServeMux()
	registerRoutes(mux, db)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := "0.0.0.0:" + port
	log.Println("Listening on", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal("server failed:", err)
	}

}

/* ======================
   Routes
   ====================== */

func registerRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/health", healthHandler(db))
	mux.HandleFunc("/player", playerHandler(db))
	mux.HandleFunc("/seasons", seasonsHandler(db))
	mux.HandleFunc("/events", eventsHandler(db))
	mux.HandleFunc("/buy-star", buyStarHandler(db))
	mux.HandleFunc("/buy-star/quote", buyStarQuoteHandler(db))
	mux.HandleFunc("/buy-variant-star", buyVariantStarHandler(db))
	mux.HandleFunc("/buy-boost", buyBoostHandler(db))
	mux.HandleFunc("/burn-coins", burnCoinsHandler(db))
	mux.HandleFunc("/claim-daily", dailyClaimHandler(db))
	mux.HandleFunc("/claim-activity", activityClaimHandler(db))
	mux.HandleFunc("/auth/signup", signupHandler(db))
	mux.HandleFunc("/auth/login", loginHandler(db))
	mux.HandleFunc("/auth/logout", logoutHandler(db))
	mux.HandleFunc("/auth/me", meHandler(db))
	mux.HandleFunc("/auth/request-reset", requestPasswordResetHandler(db))
	mux.HandleFunc("/auth/reset-password", resetPasswordHandler(db))
	mux.HandleFunc("/admin/bootstrap/status", adminBootstrapStatusHandler(db))
	mux.HandleFunc("/admin/bootstrap/claim", adminBootstrapClaimHandler(db))

	mux.HandleFunc("/notifications", notificationsHandler(db))
	mux.HandleFunc("/notifications/ack", notificationsAckHandler(db))
	mux.HandleFunc("/notifications/delete", notificationsDeleteHandler(db))
	mux.HandleFunc("/notifications/settings", notificationsSettingsHandler(db))
	mux.HandleFunc("/notifications/stream", notificationsStreamHandler(db))
	mux.HandleFunc("/activity", activityHandler(db))
	mux.HandleFunc("/profile", profileHandler(db))
	mux.HandleFunc("/telemetry", telemetryHandler(db))
	mux.HandleFunc("/admin/telemetry", adminTelemetryHandler(db))
	mux.HandleFunc("/admin/abuse-events", adminAbuseEventsHandler(db))
	mux.HandleFunc("/admin/overview", adminOverviewHandler(db))
	mux.HandleFunc("/admin/anti-cheat", adminAntiCheatHandler(db))
	mux.HandleFunc("/admin/economy", adminEconomyHandler(db))
	mux.HandleFunc("/admin/seasons/", adminSeasonControlsHandler(db))
	mux.HandleFunc("/admin/player-search", adminPlayerSearchHandler(db))
	mux.HandleFunc("/admin/audit-log", adminAuditLogHandler(db))
	mux.HandleFunc("/admin/role", adminRoleHandler(db))

	mux.HandleFunc("/admin/notifications", adminNotificationsHandler(db))
	mux.HandleFunc("/admin/player-controls", adminPlayerControlsHandler(db))
	mux.HandleFunc("/admin/settings", adminSettingsHandler(db))
	mux.HandleFunc("/admin/star-purchases", adminStarPurchaseLogHandler(db))
	mux.HandleFunc("/admin/bots", adminBotListHandler(db))
	mux.HandleFunc("/admin/bots/create", adminBotCreateHandler(db))
	mux.HandleFunc("/admin/bots/delete", adminBotDeleteHandler(db))
	mux.HandleFunc("/admin/profile-actions", adminProfileActionHandler(db))
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

	params := economy.Calibration()
	if activeDripInterval <= 0 {
		activeDripInterval = time.Duration(params.PassiveActiveIntervalSeconds) * time.Second
	}
	if idleDripInterval <= 0 {
		idleDripInterval = time.Duration(params.PassiveIdleIntervalSeconds) * time.Second
	}
	if activeDripAmount <= 0 {
		activeDripAmount = params.PassiveActiveAmount
	}
	if idleDripAmount <= 0 {
		idleDripAmount = params.PassiveIdleAmount
	}
	if idleDripAmount < 1 {
		idleDripAmount = 1
	}

	scaling := currentFaucetScaling(now)
	activeDripInterval = applyFaucetCooldownScaling(activeDripInterval, scaling.CooldownMultiplier)
	idleDripInterval = applyFaucetCooldownScaling(idleDripInterval, scaling.CooldownMultiplier)
	activeDripAmount = applyFaucetRewardScaling(activeDripAmount, scaling.RewardMultiplier)
	idleDripAmount = applyFaucetRewardScaling(idleDripAmount, scaling.RewardMultiplier)

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

		enforcement := abuseEffectiveEnforcement(db, playerID, bulkStarMaxQty())
		dripAmount = abuseAdjustedReward(dripAmount, enforcement.EarnMultiplier)

		if now.Sub(lastGrant) < dripInterval {
			continue
		}

		adjusted := int(float64(dripAmount) * dripMultiplier)
		if adjusted < 1 {
			adjusted = 1
		}
		adjusted, err = ApplyIPDampeningReward(db, playerID, adjusted)
		if err != nil {
			log.Println("drip ip dampening failed:", err)
			continue
		}
		remainingCap, err := RemainingDailyCap(db, playerID, now)
		if err != nil {
			continue
		}
		if remainingCap <= 0 {
			continue
		}
		if adjusted > remainingCap {
			adjusted = remainingCap
		}
		if !economy.TryDistributeCoins(adjusted) {
			emitServerTelemetryWithCooldown(db, nil, playerID, "faucet_denied", map[string]interface{}{
				"faucet":         FaucetPassive,
				"reason":         "EMISSION_EXHAUSTED",
				"attempted":      adjusted,
				"availableCoins": economy.AvailableCoins(),
			}, 2*time.Minute)
			emitNotification(db, NotificationInput{
				RecipientRole: NotificationRoleAdmin,
				Category:      NotificationCategoryEconomy,
				Type:          "emission_exhausted_passive",
				Priority:      NotificationPriorityHigh,
				Message:       "Passive drip blocked by emission pool exhaustion.",
				Payload: map[string]interface{}{
					"attempted": adjusted,
				},
				DedupKey:    "emission_exhausted_passive",
				DedupWindow: 30 * time.Minute,
			})
			return
		}
		if _, _, err := GrantCoinsWithCap(db, playerID, adjusted, now, FaucetPassive, nil); err != nil {
			log.Println("drip update failed:", err)
			continue
		}
		emitServerTelemetry(db, nil, playerID, "faucet_claim", map[string]interface{}{
			"faucet":         FaucetPassive,
			"granted":        adjusted,
			"attempted":      adjusted,
			"remainingCap":   remainingCap,
			"availableCoins": economy.AvailableCoins(),
		})
	}
}
