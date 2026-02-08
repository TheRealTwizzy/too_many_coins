package main

import (
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type AdminTelemetrySeriesPoint struct {
	Bucket    time.Time `json:"bucket"`
	EventType string    `json:"eventType"`
	Count     int       `json:"count"`
}

type AdminTelemetryResponse struct {
	OK     bool                        `json:"ok"`
	Error  string                      `json:"error,omitempty"`
	Series []AdminTelemetrySeriesPoint `json:"series,omitempty"`
}

type AdminEconomyResponse struct {
	OK                  bool    `json:"ok"`
	Error               string  `json:"error,omitempty"`
	DailyEmissionTarget int     `json:"dailyEmissionTarget,omitempty"`
	BaseStarPrice       int     `json:"baseStarPrice,omitempty"`
	CurrentStarPrice    int     `json:"currentStarPrice,omitempty"`
	MarketPressure      float64 `json:"marketPressure,omitempty"`
	DailyCapEarly       int     `json:"dailyCapEarly,omitempty"`
	DailyCapLate        int     `json:"dailyCapLate,omitempty"`
	FaucetsEnabled      bool    `json:"faucetsEnabled,omitempty"`
	SinksEnabled        bool    `json:"sinksEnabled,omitempty"`
	TelemetryEnabled    bool    `json:"telemetryEnabled,omitempty"`
}

type AdminEconomyUpdateRequest struct {
	DailyEmissionTarget *int  `json:"dailyEmissionTarget,omitempty"`
	FaucetsEnabled      *bool `json:"faucetsEnabled,omitempty"`
	SinksEnabled        *bool `json:"sinksEnabled,omitempty"`
	TelemetryEnabled    *bool `json:"telemetryEnabled,omitempty"`
}

type AdminStarPurchaseLogItem struct {
	ID           int64     `json:"id"`
	AccountID    string    `json:"accountId"`
	PlayerID     string    `json:"playerId"`
	SeasonID     string    `json:"seasonId"`
	PurchaseType string    `json:"purchaseType"`
	Variant      string    `json:"variant,omitempty"`
	PricePaid    int64     `json:"pricePaid"`
	CoinsBefore  int64     `json:"coinsBefore"`
	CoinsAfter   int64     `json:"coinsAfter"`
	StarsBefore  int64     `json:"starsBefore"`
	StarsAfter   int64     `json:"starsAfter"`
	CreatedAt    time.Time `json:"createdAt"`
}

type AdminStarPurchaseLogResponse struct {
	OK    bool                       `json:"ok"`
	Error string                     `json:"error,omitempty"`
	Items []AdminStarPurchaseLogItem `json:"items,omitempty"`
}

type AdminAbuseEvent struct {
	ID         int64           `json:"id"`
	AccountID  string          `json:"accountId,omitempty"`
	PlayerID   string          `json:"playerId,omitempty"`
	SeasonID   string          `json:"seasonId,omitempty"`
	EventType  string          `json:"eventType"`
	Severity   int             `json:"severity"`
	ScoreDelta float64         `json:"scoreDelta"`
	Details    json.RawMessage `json:"details,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
}

type AdminAbuseEventsResponse struct {
	OK    bool              `json:"ok"`
	Error string            `json:"error,omitempty"`
	Items []AdminAbuseEvent `json:"items,omitempty"`
}

type AdminOverviewResponse struct {
	OK                   bool    `json:"ok"`
	Error                string  `json:"error,omitempty"`
	ActiveSeasons        int     `json:"activeSeasons"`
	CoinsEmittedLastHour int64   `json:"coinsEmittedLastHour"`
	StarsPurchasedHour   int64   `json:"starsPurchasedLastHour"`
	MarketPressure       float64 `json:"marketPressure"`
	MarketPressureRatio  float64 `json:"marketPressureRatio"`
	ActiveThrottles      int     `json:"activeThrottles"`
	ActiveAbuseFlags     int     `json:"activeAbuseFlags"`
	AbuseEventsLastHour  int     `json:"abuseEventsLastHour"`
	AbuseSevereLastHour  int     `json:"abuseSevereLastHour"`
}

type AdminToggleStatus struct {
	Key             string     `json:"key"`
	Label           string     `json:"label"`
	Status          string     `json:"status"`
	LastTriggeredAt *time.Time `json:"lastTriggeredAt,omitempty"`
	EventCount      int        `json:"eventCount"`
}

type AdminAntiCheatResponse struct {
	OK          bool                `json:"ok"`
	Error       string              `json:"error,omitempty"`
	Toggles     []AdminToggleStatus `json:"toggles,omitempty"`
	Sensitivity map[string]string   `json:"sensitivity,omitempty"`
}

type AdminPlayerSearchItem struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	PlayerID    string `json:"playerId"`
	AccountID   string `json:"accountId"`
	TrustStatus string `json:"trustStatus"`
	SeasonID    string `json:"seasonId"`
	FlagCount   int    `json:"flagCount"`
}

type AdminPlayerSearchResponse struct {
	OK     bool                    `json:"ok"`
	Error  string                  `json:"error,omitempty"`
	Items  []AdminPlayerSearchItem `json:"items,omitempty"`
	Total  int                     `json:"total"`
	Limit  int                     `json:"limit"`
	Query  string                  `json:"query,omitempty"`
	Status string                  `json:"trustStatus,omitempty"`
}

type AdminAuditLogItem struct {
	ID            int64           `json:"id"`
	AdminAccount  string          `json:"adminAccount"`
	AdminUsername string          `json:"adminUsername"`
	ActionType    string          `json:"actionType"`
	ScopeType     string          `json:"scopeType"`
	ScopeID       string          `json:"scopeId"`
	Reason        string          `json:"reason,omitempty"`
	Details       json.RawMessage `json:"details,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type AdminAuditLogResponse struct {
	OK    bool                `json:"ok"`
	Error string              `json:"error,omitempty"`
	Items []AdminAuditLogItem `json:"items,omitempty"`
	Total int                 `json:"total"`
	Limit int                 `json:"limit"`
	Query string              `json:"query,omitempty"`
}

func requireAdmin(db *sql.DB, w http.ResponseWriter, r *http.Request) (*Account, bool) {
	account, _, err := getSessionAccount(db, r)
	if err != nil || account == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, false
	}
	if account.Role != "admin" {
		w.WriteHeader(http.StatusForbidden)
		return nil, false
	}
	if account.MustChangePassword {
		w.WriteHeader(http.StatusForbidden)
		return nil, false
	}
	return account, true
}

func requireModerator(db *sql.DB, w http.ResponseWriter, r *http.Request) (*Account, bool) {
	account, _, err := getSessionAccount(db, r)
	if err != nil || account == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, false
	}
	if account.Role != "admin" && account.Role != "moderator" {
		w.WriteHeader(http.StatusForbidden)
		return nil, false
	}
	if account.Role == "admin" && account.MustChangePassword {
		w.WriteHeader(http.StatusForbidden)
		return nil, false
	}
	return account, true
}

func adminTelemetryHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}

		rows, err := db.Query(`
			SELECT date_trunc('hour', created_at) AS bucket, event_type, COUNT(*)
			FROM player_telemetry
			WHERE created_at >= NOW() - INTERVAL '48 hours'
			GROUP BY bucket, event_type
			ORDER BY bucket ASC
		`)
		if err != nil {
			json.NewEncoder(w).Encode(AdminTelemetryResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer rows.Close()

		series := []AdminTelemetrySeriesPoint{}
		for rows.Next() {
			var point AdminTelemetrySeriesPoint
			if err := rows.Scan(&point.Bucket, &point.EventType, &point.Count); err != nil {
				continue
			}
			series = append(series, point)
		}

		json.NewEncoder(w).Encode(AdminTelemetryResponse{
			OK:     true,
			Series: series,
		})
	}
}

func adminEconomyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}

		switch r.Method {
		case http.MethodGet:
			params := economy.Calibration()
			coins := economy.CoinsInCirculation()
			remaining := seasonSecondsRemaining(time.Now().UTC())
			json.NewEncoder(w).Encode(AdminEconomyResponse{
				OK:                  true,
				DailyEmissionTarget: economy.DailyEmissionTarget(),
				BaseStarPrice:       params.P0,
				CurrentStarPrice:    ComputeSeasonAuthorityStarPrice(coins, remaining),
				MarketPressure:      economy.MarketPressure(),
				DailyCapEarly:       params.DailyCapEarly,
				DailyCapLate:        params.DailyCapLate,
				FaucetsEnabled:      featureFlags.FaucetsEnabled,
				SinksEnabled:        featureFlags.SinksEnabled,
				TelemetryEnabled:    featureFlags.Telemetry,
			})
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}
}

type AdminSeasonControlItem struct {
	ControlName    string          `json:"controlName"`
	Value          json.RawMessage `json:"value"`
	ExpiresAt      *time.Time      `json:"expiresAt,omitempty"`
	LastModifiedAt time.Time       `json:"lastModifiedAt"`
	LastModifiedBy string          `json:"lastModifiedBy"`
}

type AdminSeasonControlsResponse struct {
	OK    bool                     `json:"ok"`
	Error string                   `json:"error,omitempty"`
	Items []AdminSeasonControlItem `json:"items,omitempty"`
	Item  *AdminSeasonControlItem  `json:"item,omitempty"`
}

type AdminSeasonAdvanceResponse struct {
	OK              bool   `json:"ok"`
	Error           string `json:"error,omitempty"`
	SeasonID        string `json:"seasonId,omitempty"`
	SeasonStartTime string `json:"seasonStartTime,omitempty"`
	SeasonEndTime   string `json:"seasonEndTime,omitempty"`
	TotalDays       int    `json:"totalDays,omitempty"`
}

type AdminSeasonRecoveryRequest struct {
	Confirm string `json:"confirm"`
}

type AdminSeasonRecoveryResponse struct {
	OK              bool   `json:"ok"`
	Error           string `json:"error,omitempty"`
	SeasonID        string `json:"seasonId,omitempty"`
	SeasonStartTime string `json:"seasonStartTime,omitempty"`
	SeasonEndTime   string `json:"seasonEndTime,omitempty"`
	TotalDays       int    `json:"totalDays,omitempty"`
	PriorSeasonID   string `json:"priorSeasonId,omitempty"`
}

type AdminSeasonControlRequest struct {
	ControlName string          `json:"controlName"`
	Value       json.RawMessage `json:"value"`
	ExpiresAt   *string         `json:"expiresAt,omitempty"`
	Reason      string          `json:"reason"`
	Intent      string          `json:"intent"`
}

const (
	seasonControlFreeze             = "SEASON_FREEZE"
	seasonControlEmissionMultiplier = "EMISSION_MULTIPLIER"
	seasonControlExtensionDays      = "SEASON_EXTENSION_DAYS"
	seasonControlPressureClamp      = "MARKET_PRESSURE_RATE_CLAMP"
)

var allowedSeasonControlIntents = map[string]bool{
	"EMERGENCY_FREEZE":       true,
	"STARVATION_PREVENTION":  true,
	"FLOOD_CONTROL":          true,
	"VOLATILITY_DAMPENING":   true,
	"TELEMETRY_PRESERVATION": true,
}

func adminSeasonControlsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seasonID, ok := parseSeasonControlsPath(r.URL.Path)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodPost:
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		adminAccount, ok := requireAdmin(db, w, r)
		if !ok {
			return
		}

		seasonUUID := normalizeUUIDFromString(seasonID, "season")

		if r.Method == http.MethodGet {
			rows, err := db.Query(`
				SELECT control_name, value, expires_at, last_modified_at, last_modified_by
				FROM season_controls
				WHERE season_id = $1
				ORDER BY control_name ASC
			`, seasonUUID)
			if err != nil {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			defer rows.Close()

			items := []AdminSeasonControlItem{}
			for rows.Next() {
				var item AdminSeasonControlItem
				var value []byte
				var expiresAt sql.NullTime
				if err := rows.Scan(&item.ControlName, &value, &expiresAt, &item.LastModifiedAt, &item.LastModifiedBy); err != nil {
					continue
				}
				item.Value = json.RawMessage(value)
				if expiresAt.Valid {
					exp := expiresAt.Time.UTC()
					item.ExpiresAt = &exp
				}
				items = append(items, item)
			}

			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: true, Items: items})
			return
		}

		var req AdminSeasonControlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		controlName := strings.TrimSpace(req.ControlName)
		reason := strings.TrimSpace(req.Reason)
		intent := strings.TrimSpace(req.Intent)
		if controlName == "" || reason == "" || intent == "" {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		if len(reason) < 20 {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_REASON"})
			return
		}
		if !allowedSeasonControlIntents[intent] {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_INTENT"})
			return
		}
		if len(req.Value) == 0 || strings.TrimSpace(string(req.Value)) == "null" {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_VALUE"})
			return
		}

		now := time.Now().UTC()
		var expiresAt *time.Time
		if req.ExpiresAt != nil && strings.TrimSpace(*req.ExpiresAt) != "" {
			parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.ExpiresAt))
			if err != nil {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_EXPIRES_AT"})
				return
			}
			value := parsed.UTC()
			expiresAt = &value
		}

		var normalizedValue json.RawMessage
		var normalizedInterface interface{}
		switch controlName {
		case seasonControlFreeze:
			var value bool
			if err := json.Unmarshal(req.Value, &value); err != nil {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_VALUE"})
				return
			}
			payload, _ := json.Marshal(value)
			normalizedValue = payload
			normalizedInterface = value
		case seasonControlEmissionMultiplier:
			var value float64
			if err := json.Unmarshal(req.Value, &value); err != nil {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_VALUE"})
				return
			}
			if value < 0.5 || value > 1.5 {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "OUT_OF_BOUNDS"})
				return
			}
			rounded := math.Round(value*100) / 100
			if rounded < 0.5 || rounded > 1.5 {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "OUT_OF_BOUNDS"})
				return
			}
			payload, _ := json.Marshal(rounded)
			normalizedValue = payload
			normalizedInterface = rounded
		case seasonControlExtensionDays:
			var value float64
			if err := json.Unmarshal(req.Value, &value); err != nil {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_VALUE"})
				return
			}
			if value != math.Trunc(value) {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_VALUE"})
				return
			}
			days := int(value)
			if days < 0 || days > 7 {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "OUT_OF_BOUNDS"})
				return
			}
			if alphaSeasonLengthDays+days > alphaSeasonMaxDays {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "OUT_OF_BOUNDS"})
				return
			}
			payload, _ := json.Marshal(days)
			normalizedValue = payload
			normalizedInterface = days
		case seasonControlPressureClamp:
			if expiresAt == nil {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "MISSING_EXPIRES_AT"})
				return
			}
			expiry := *expiresAt
			if expiry.Before(now) || expiry.After(now.Add(48*time.Hour)) {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_EXPIRES_AT"})
				return
			}
			var value float64
			if err := json.Unmarshal(req.Value, &value); err != nil {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_VALUE"})
				return
			}
			if value < 0.5 || value > 1.0 {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "OUT_OF_BOUNDS"})
				return
			}
			payload, _ := json.Marshal(value)
			normalizedValue = payload
			normalizedInterface = value
		default:
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INVALID_CONTROL"})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer tx.Rollback()

		var existingValue []byte
		var existingExpires sql.NullTime
		var existingModified time.Time
		exists := false
		err = tx.QueryRow(`
			SELECT value, expires_at, last_modified_at
			FROM season_controls
			WHERE season_id = $1 AND control_name = $2
		`, seasonUUID, controlName).Scan(&existingValue, &existingExpires, &existingModified)
		if err == nil {
			exists = true
		} else if err != sql.ErrNoRows {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		switch controlName {
		case seasonControlEmissionMultiplier:
			if exists && now.Sub(existingModified) < 24*time.Hour {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "COOLDOWN_ACTIVE"})
				return
			}
		case seasonControlExtensionDays:
			if exists {
				json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "COOLDOWN_ACTIVE"})
				return
			}
		case seasonControlPressureClamp:
			if exists {
				if !existingExpires.Valid || existingExpires.Time.After(now) {
					json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "COOLDOWN_ACTIVE"})
					return
				}
			}
		}

		adminUUID := normalizeUUIDFromString(adminAccount.AccountID, "admin")

		_, err = tx.Exec(`
			INSERT INTO season_controls (season_id, control_name, value, expires_at, last_modified_by)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (season_id, control_name)
			DO UPDATE SET value = EXCLUDED.value,
				expires_at = EXCLUDED.expires_at,
				last_modified_at = NOW(),
				last_modified_by = EXCLUDED.last_modified_by
		`, seasonUUID, controlName, normalizedValue, expiresAt, adminUUID)
		if err != nil {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		var updated AdminSeasonControlItem
		var updatedValue []byte
		var updatedExpires sql.NullTime
		if err := tx.QueryRow(`
			SELECT control_name, value, expires_at, last_modified_at, last_modified_by
			FROM season_controls
			WHERE season_id = $1 AND control_name = $2
		`, seasonUUID, controlName).Scan(&updated.ControlName, &updatedValue, &updatedExpires, &updated.LastModifiedAt, &updated.LastModifiedBy); err != nil {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		updated.Value = json.RawMessage(updatedValue)
		if updatedExpires.Valid {
			value := updatedExpires.Time.UTC()
			updated.ExpiresAt = &value
		}

		oldValue := interface{}(nil)
		if exists {
			if len(existingValue) > 0 {
				var decoded interface{}
				if err := json.Unmarshal(existingValue, &decoded); err == nil {
					oldValue = decoded
				} else {
					oldValue = string(existingValue)
				}
			}
		}

		eventID := generateRandomUUID()
		dayIndex := seasonDayIndex(now)
		coinsInCirculation := economy.CoinsInCirculation()
		activePlayers := economy.ActivePlayers()
		marketPressure := economy.MarketPressure()

		var emissionPoolRemaining int64
		var globalCoinPool int64
		var coinsDistributed int64
		_ = db.QueryRow(`
			SELECT COALESCE(global_coin_pool, 0), COALESCE(coins_distributed, 0)
			FROM season_economy
			WHERE season_id = $1
		`, currentSeasonID()).Scan(&globalCoinPool, &coinsDistributed)
		emissionPoolRemaining = globalCoinPool - coinsDistributed
		if emissionPoolRemaining < 0 {
			emissionPoolRemaining = 0
		}

		_, err = tx.Exec(`
			INSERT INTO season_control_events (
				event_id,
				season_id,
				control_name,
				intent,
				old_value,
				new_value,
				reason,
				season_day_index,
				coins_in_circulation_snapshot,
				active_players_snapshot,
				market_pressure_snapshot,
				emission_pool_snapshot,
				created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
		`, eventID, seasonUUID, controlName, intent, existingValue, normalizedValue, reason,
			dayIndex, coinsInCirculation, activePlayers, marketPressure, emissionPoolRemaining)
		if err != nil {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		payload := map[string]interface{}{
			"seasonId":    seasonID,
			"controlName": controlName,
			"intent":      intent,
			"oldValue":    oldValue,
			"newValue":    normalizedInterface,
		}
		if expiresAt != nil {
			payload["expiresAt"] = expiresAt.Format(time.RFC3339)
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		_, err = tx.Exec(`
			INSERT INTO admin_audit_log (admin_account_id, action_type, scope_type, scope_id, reason, details, created_at)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, NOW())
		`, adminAccount.AccountID, "season_control_set", "season_control", seasonID+":"+controlName, reason, string(payloadBytes))
		if err != nil {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		if err := tx.Commit(); err != nil {
			json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		telemetryPayload := map[string]interface{}{
			"eventId":                eventID,
			"seasonId":               seasonUUID,
			"controlName":            controlName,
			"intent":                 intent,
			"reason":                 reason,
			"oldValue":               oldValue,
			"newValue":               normalizedInterface,
			"dayIndex":               dayIndex,
			"coinsSnapshot":          coinsInCirculation,
			"activePlayersSnapshot":  activePlayers,
			"marketPressureSnapshot": marketPressure,
			"emissionPoolSnapshot":   emissionPoolRemaining,
		}
		telemetryBytes, _ := json.Marshal(telemetryPayload)
		_, _ = db.Exec(`
			INSERT INTO player_telemetry (account_id, player_id, event_type, payload, created_at)
			VALUES ($1, $2, $3, $4, NOW())
		`, adminAccount.AccountID, "", "admin_season_control", telemetryBytes)

		json.NewEncoder(w).Encode(AdminSeasonControlsResponse{OK: true, Item: &updated})
	}
}

func adminSeasonAdvanceHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		adminAccount, ok := requireAdmin(db, w, r)
		if !ok {
			return
		}
		if CurrentPhase() != PhaseAlpha {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(AdminSeasonAdvanceResponse{OK: false, Error: "INVALID_PHASE"})
			return
		}
		if r.Body != nil {
			var payload interface{}
			decoder := json.NewDecoder(r.Body)
			if err := decoder.Decode(&payload); err == nil || err != io.EOF {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(AdminSeasonAdvanceResponse{OK: false, Error: "INVALID_REQUEST"})
				return
			}
		}

		now := time.Now().UTC()
		previous, hasPrevious := ActiveSeasonState()
		if hasPrevious && !isSeasonEndedRaw(now) {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(AdminSeasonAdvanceResponse{OK: false, Error: "SEASON_ACTIVE"})
			return
		}

		if err := AdvanceAlphaSeasonOverride(db, now); err != nil {
			if errors.Is(err, ErrActiveSeason) {
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(AdminSeasonAdvanceResponse{OK: false, Error: "SEASON_ACTIVE"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AdminSeasonAdvanceResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		current, ok := ActiveSeasonState()
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AdminSeasonAdvanceResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		totalDays := int(seasonLength().Hours() / 24)
		if totalDays < 1 {
			totalDays = 1
		}
		startTime := current.StartUTC.UTC()
		endTime := startTime.Add(seasonLength()).UTC()

		_ = logAdminAction(db, adminAccount.AccountID, "season_advance", "season", current.SeasonID, "override", map[string]interface{}{
			"previousSeasonId": func() string {
				if hasPrevious {
					return previous.SeasonID
				}
				return ""
			}(),
			"startedAt": startTime.Format(time.RFC3339),
		})

		json.NewEncoder(w).Encode(AdminSeasonAdvanceResponse{
			OK:              true,
			SeasonID:        current.SeasonID,
			SeasonStartTime: startTime.Format(time.RFC3339),
			SeasonEndTime:   endTime.Format(time.RFC3339),
			TotalDays:       totalDays,
		})
	}
}

func adminSeasonRecoveryHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		adminAccount, ok := requireAdmin(db, w, r)
		if !ok {
			return
		}
		if CurrentPhase() != PhaseAlpha {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(AdminSeasonRecoveryResponse{OK: false, Error: "INVALID_PHASE"})
			return
		}

		var req AdminSeasonRecoveryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(AdminSeasonRecoveryResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		// Require explicit confirmation text
		if strings.TrimSpace(req.Confirm) != "I understand this is a recovery action" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(AdminSeasonRecoveryResponse{OK: false, Error: "INVALID_CONFIRMATION"})
			return
		}

		now := time.Now().UTC()

		// Check that the current season exists and has ended
		current, hasCurrent := ActiveSeasonState()
		if !hasCurrent {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(AdminSeasonRecoveryResponse{OK: false, Error: "NO_ACTIVE_SEASON"})
			return
		}

		if !isSeasonEndedRaw(now) {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(AdminSeasonRecoveryResponse{OK: false, Error: "SEASON_ACTIVE"})
			return
		}

		// Call the existing advancement logic to trigger rollover
		if err := AdvanceAlphaSeasonOverride(db, now); err != nil {
			if errors.Is(err, ErrActiveSeason) {
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(AdminSeasonRecoveryResponse{OK: false, Error: "SEASON_ACTIVE"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AdminSeasonRecoveryResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		// Get the new season details
		newSeason, ok := ActiveSeasonState()
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AdminSeasonRecoveryResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		totalDays := int(seasonLength().Hours() / 24)
		if totalDays < 1 {
			totalDays = 1
		}
		startTime := newSeason.StartUTC.UTC()
		endTime := startTime.Add(seasonLength()).UTC()

		// Log to admin audit log
		_ = logAdminAction(db, adminAccount.AccountID, "season_recovery", "season", newSeason.SeasonID, "recovery-only", map[string]interface{}{
			"recoveredFromSeasonId": current.SeasonID,
			"recoveredFromEndTime":  current.StartUTC.Add(seasonLength()).Format(time.RFC3339),
			"newSeasonId":           newSeason.SeasonID,
			"newSeasonStartedAt":    startTime.Format(time.RFC3339),
		})

		json.NewEncoder(w).Encode(AdminSeasonRecoveryResponse{
			OK:              true,
			SeasonID:        newSeason.SeasonID,
			SeasonStartTime: startTime.Format(time.RFC3339),
			SeasonEndTime:   endTime.Format(time.RFC3339),
			TotalDays:       totalDays,
			PriorSeasonID:   current.SeasonID,
		})
	}
}

func adminAbuseEventsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}

		rows, err := db.Query(`
			SELECT id, COALESCE(account_id, ''), COALESCE(player_id, ''), COALESCE(season_id, ''), event_type, severity, score_delta, details, created_at
			FROM abuse_events
			ORDER BY created_at DESC
			LIMIT 200
		`)
		if err != nil {
			json.NewEncoder(w).Encode(AdminAbuseEventsResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer rows.Close()

		items := []AdminAbuseEvent{}
		for rows.Next() {
			var item AdminAbuseEvent
			var details sql.NullString
			if err := rows.Scan(
				&item.ID,
				&item.AccountID,
				&item.PlayerID,
				&item.SeasonID,
				&item.EventType,
				&item.Severity,
				&item.ScoreDelta,
				&details,
				&item.CreatedAt,
			); err != nil {
				continue
			}
			if details.Valid {
				item.Details = json.RawMessage(details.String)
			}
			items = append(items, item)
		}

		json.NewEncoder(w).Encode(AdminAbuseEventsResponse{OK: true, Items: items})
	}
}

func adminOverviewHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}

		now := time.Now().UTC()
		activeSeasons := 1
		if isSeasonEnded(now) {
			activeSeasons = 0
		}

		var coinsEmitted int64
		_ = db.QueryRow(`
			SELECT COALESCE(SUM(amount), 0)
			FROM coin_earning_log
			WHERE created_at >= $1
		`, now.Add(-1*time.Hour)).Scan(&coinsEmitted)

		var starsPurchased int64
		_ = db.QueryRow(`
			SELECT COUNT(*)
			FROM star_purchase_log
			WHERE created_at >= $1
		`, now.Add(-1*time.Hour)).Scan(&starsPurchased)

		var last24h int64
		var last7d int64
		_ = db.QueryRow(`
			SELECT COUNT(*)
			FROM star_purchase_log
			WHERE season_id = $1 AND created_at >= $2
		`, currentSeasonID(), now.Add(-24*time.Hour)).Scan(&last24h)
		_ = db.QueryRow(`
			SELECT COUNT(*)
			FROM star_purchase_log
			WHERE season_id = $1 AND created_at >= $2
		`, currentSeasonID(), now.Add(-7*24*time.Hour)).Scan(&last7d)
		marketRatio := 0.0
		if last7d > 0 {
			marketRatio = float64(last24h) / (float64(last7d) / 7.0)
		}

		var activeThrottles int
		_ = db.QueryRow(`
			SELECT COUNT(*)
			FROM player_abuse_state
			WHERE season_id = $1 AND severity >= 1
		`, currentSeasonID()).Scan(&activeThrottles)

		var activeFlags int
		_ = db.QueryRow(`
			SELECT COUNT(*)
			FROM player_abuse_state
			WHERE season_id = $1 AND severity >= 2
		`, currentSeasonID()).Scan(&activeFlags)

		var abuseEvents int
		_ = db.QueryRow(`
			SELECT COUNT(*)
			FROM abuse_events
			WHERE created_at >= $1
		`, now.Add(-1*time.Hour)).Scan(&abuseEvents)

		var abuseSevere int
		_ = db.QueryRow(`
			SELECT COUNT(*)
			FROM abuse_events
			WHERE created_at >= $1 AND severity >= 3
		`, now.Add(-1*time.Hour)).Scan(&abuseSevere)

		json.NewEncoder(w).Encode(AdminOverviewResponse{
			OK:                   true,
			ActiveSeasons:        activeSeasons,
			CoinsEmittedLastHour: coinsEmitted,
			StarsPurchasedHour:   starsPurchased,
			MarketPressure:       economy.MarketPressure(),
			MarketPressureRatio:  marketRatio,
			ActiveThrottles:      activeThrottles,
			ActiveAbuseFlags:     activeFlags,
			AbuseEventsLastHour:  abuseEvents,
			AbuseSevereLastHour:  abuseSevere,
		})
	}
}

func adminAntiCheatHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}

		now := time.Now().UTC()
		queryStats := func(eventTypes []string) (int, *time.Time) {
			if len(eventTypes) == 0 {
				return 0, nil
			}
			args := []interface{}{now.Add(-24 * time.Hour)}
			placeholders := []string{}
			for i, eventType := range eventTypes {
				args = append(args, eventType)
				placeholders = append(placeholders, "$"+strconv.Itoa(i+2))
			}
			inClause := strings.Join(placeholders, ", ")
			countQuery := `SELECT COUNT(*) FROM abuse_events WHERE created_at >= $1 AND event_type IN (` + inClause + `)`
			var count int
			_ = db.QueryRow(countQuery, args...).Scan(&count)

			lastQuery := `SELECT created_at FROM abuse_events WHERE event_type IN (` + inClause + `) ORDER BY created_at DESC LIMIT 1`
			var last time.Time
			if err := db.QueryRow(lastQuery, args[1:]...).Scan(&last); err == nil {
				return count, &last
			}
			return count, nil
		}

		ipCount, ipLast := queryStats([]string{"ip_cluster_activity"})
		abuseCount, abuseLast := queryStats([]string{"purchase_burst", "purchase_regular_interval", "activity_regular_interval", "tick_reaction_burst", "ip_cluster_activity"})

		toggles := []AdminToggleStatus{
			{
				Key:             "ip_enforcement",
				Label:           "Enable IP enforcement",
				Status:          map[bool]string{true: "enabled", false: "disabled"}[featureFlags.IPThrottling],
				LastTriggeredAt: ipLast,
				EventCount:      ipCount,
			},
			{
				Key:             "clustering_detection",
				Label:           "Enable clustering detection",
				Status:          "always-on",
				LastTriggeredAt: ipLast,
				EventCount:      ipCount,
			},
			{
				Key:             "automatic_throttling",
				Label:           "Enable automatic throttling",
				Status:          "always-on",
				LastTriggeredAt: abuseLast,
				EventCount:      abuseCount,
			},
			{
				Key:             "trade_tightening",
				Label:           "Enable trade eligibility tightening",
				Status:          "not-configured",
				LastTriggeredAt: nil,
				EventCount:      0,
			},
			{
				Key:             "bot_detection",
				Label:           "Enable bot detection heuristics",
				Status:          "not-configured",
				LastTriggeredAt: nil,
				EventCount:      0,
			},
		}

		json.NewEncoder(w).Encode(AdminAntiCheatResponse{
			OK:      true,
			Toggles: toggles,
			Sensitivity: map[string]string{
				"clustering": "not-configured",
				"throttle":   "not-configured",
				"trade":      "not-configured",
				"faucet":     "not-configured",
			},
		})
	}
}

func adminPlayerSearchHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		trust := strings.TrimSpace(r.URL.Query().Get("trust"))
		limit := 25
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 100 {
			limit = 100
		}

		clauses := []string{}
		args := []interface{}{}
		argIndex := 1
		if query != "" {
			clauses = append(clauses, "(a.username ILIKE $"+strconv.Itoa(argIndex)+" OR a.player_id = $"+strconv.Itoa(argIndex+1)+" OR a.account_id = $"+strconv.Itoa(argIndex+2)+")")
			args = append(args, "%"+strings.ToLower(query)+"%", query, query)
			argIndex += 3
		}
		if trust != "" {
			clauses = append(clauses, "a.trust_status = $"+strconv.Itoa(argIndex))
			args = append(args, trust)
			argIndex++
		}

		where := ""
		if len(clauses) > 0 {
			where = "WHERE " + strings.Join(clauses, " AND ")
		}

		sqlQuery := `
			SELECT a.username, a.display_name, a.player_id, a.account_id, a.trust_status,
				(
					SELECT COUNT(*)
					FROM abuse_events e
					WHERE e.player_id = a.player_id AND e.season_id = $` + strconv.Itoa(argIndex) + `
				) AS flag_count
			FROM accounts a
			` + where + `
			ORDER BY a.username ASC
			LIMIT ` + strconv.Itoa(limit)
		args = append(args, currentSeasonID())

		rows, err := db.Query(sqlQuery, args...)
		if err != nil {
			json.NewEncoder(w).Encode(AdminPlayerSearchResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer rows.Close()

		items := []AdminPlayerSearchItem{}
		for rows.Next() {
			var item AdminPlayerSearchItem
			if err := rows.Scan(&item.Username, &item.DisplayName, &item.PlayerID, &item.AccountID, &item.TrustStatus, &item.FlagCount); err != nil {
				continue
			}
			item.SeasonID = currentSeasonID()
			items = append(items, item)
		}

		json.NewEncoder(w).Encode(AdminPlayerSearchResponse{
			OK:     true,
			Items:  items,
			Total:  len(items),
			Limit:  limit,
			Query:  query,
			Status: trust,
		})
	}
}

func adminAuditLogHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		limit := 50
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 200 {
			limit = 200
		}

		clauses := []string{}
		args := []interface{}{}
		argIndex := 1
		if query != "" {
			clauses = append(clauses, "(a.username ILIKE $"+strconv.Itoa(argIndex)+" OR l.action_type ILIKE $"+strconv.Itoa(argIndex)+" OR l.scope_id ILIKE $"+strconv.Itoa(argIndex)+" OR l.reason ILIKE $"+strconv.Itoa(argIndex)+")")
			args = append(args, "%"+query+"%")
			argIndex++
		}
		where := ""
		if len(clauses) > 0 {
			where = "WHERE " + strings.Join(clauses, " AND ")
		}

		sqlQuery := `
			SELECT l.id, l.admin_account_id, COALESCE(a.username, ''), l.action_type, l.scope_type, l.scope_id, l.reason, l.details, l.created_at
			FROM admin_audit_log l
			LEFT JOIN accounts a ON a.account_id = l.admin_account_id
			` + where + `
			ORDER BY l.created_at DESC
			LIMIT ` + strconv.Itoa(limit)

		rows, err := db.Query(sqlQuery, args...)
		if err != nil {
			json.NewEncoder(w).Encode(AdminAuditLogResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer rows.Close()

		items := []AdminAuditLogItem{}
		for rows.Next() {
			var item AdminAuditLogItem
			var details sql.NullString
			if err := rows.Scan(
				&item.ID,
				&item.AdminAccount,
				&item.AdminUsername,
				&item.ActionType,
				&item.ScopeType,
				&item.ScopeID,
				&item.Reason,
				&details,
				&item.CreatedAt,
			); err != nil {
				continue
			}
			if details.Valid {
				item.Details = json.RawMessage(details.String)
			}
			items = append(items, item)
		}

		json.NewEncoder(w).Encode(AdminAuditLogResponse{
			OK:    true,
			Items: items,
			Total: len(items),
			Limit: limit,
			Query: query,
		})
	}
}

type AdminRoleRequest struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

type AdminRoleResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func adminRoleHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		adminAccount, ok := requireAdmin(db, w, r)
		if !ok {
			return
		}
		if strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) == "alpha" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var req AdminRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
			json.NewEncoder(w).Encode(AdminRoleResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		if err := setAccountRoleByUsername(db, req.Username, req.Role); err != nil {
			json.NewEncoder(w).Encode(AdminRoleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if normalizeRole(req.Role) == "admin" {
			var existing sql.NullString
			if err := db.QueryRow(`
				SELECT admin_key_hash FROM accounts WHERE username = $1
			`, strings.ToLower(strings.TrimSpace(req.Username))).Scan(&existing); err == nil {
				if !existing.Valid || existing.String == "" {
					generated, err := generateAdminKey()
					if err == nil {
						_ = setAdminKeyByUsername(db, req.Username, generated)
					}
				}
			}
		}
		emitNotification(db, NotificationInput{
			RecipientRole: NotificationRoleAdmin,
			Category:      NotificationCategoryAdmin,
			Type:          "role_updated",
			Priority:      NotificationPriorityNormal,
			Message:       "Role updated for @" + strings.ToLower(strings.TrimSpace(req.Username)) + ": " + normalizeRole(req.Role),
			Payload: map[string]interface{}{
				"username": strings.ToLower(strings.TrimSpace(req.Username)),
				"role":     normalizeRole(req.Role),
			},
		})
		_ = logAdminAction(db, adminAccount.AccountID, "role_update", "account", strings.ToLower(strings.TrimSpace(req.Username)), "", map[string]interface{}{
			"role": normalizeRole(req.Role),
		})
		json.NewEncoder(w).Encode(AdminRoleResponse{OK: true})
	}
}

type ModeratorProfileRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
	Bio         string `json:"bio,omitempty"`
	Pronouns    string `json:"pronouns,omitempty"`
	Location    string `json:"location,omitempty"`
	Website     string `json:"website,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

type ModeratorProfileResponse struct {
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
	Role        string `json:"role,omitempty"`
	Frozen      bool   `json:"frozen,omitempty"`
	PlayerID    string `json:"playerId,omitempty"`
}

func moderatorProfileHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer, ok := requireModerator(db, w, r)
		if !ok {
			return
		}
		switch r.Method {
		case http.MethodGet:
			username := r.URL.Query().Get("username")
			if username == "" {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "INVALID_REQUEST"})
				return
			}
			var displayName string
			var email sql.NullString
			var bio sql.NullString
			var pronouns sql.NullString
			var location sql.NullString
			var website sql.NullString
			var avatarURL sql.NullString
			var role string
			var playerID string
			err := db.QueryRow(`
				SELECT display_name, email, bio, pronouns, location, website, avatar_url, role, player_id
				FROM accounts
				WHERE username = $1
			`, strings.ToLower(username)).Scan(&displayName, &email, &bio, &pronouns, &location, &website, &avatarURL, &role, &playerID)
			if err == sql.ErrNoRows {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "NOT_FOUND"})
				return
			}
			if err != nil {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if viewer.Role == "moderator" && baseRoleFromRaw(role) == "admin" {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "FORBIDDEN"})
				return
			}
			resp := ModeratorProfileResponse{
				OK:          true,
				Username:    strings.ToLower(username),
				DisplayName: displayName,
				Role:        baseRoleFromRaw(role),
				Frozen:      isFrozenRole(role),
				PlayerID:    playerID,
			}
			if email.Valid {
				resp.Email = email.String
			}
			if bio.Valid {
				resp.Bio = bio.String
			}
			if pronouns.Valid {
				resp.Pronouns = pronouns.String
			}
			if location.Valid {
				resp.Location = location.String
			}
			if website.Valid {
				resp.Website = website.String
			}
			if avatarURL.Valid {
				resp.AvatarURL = avatarURL.String
			}
			json.NewEncoder(w).Encode(resp)
			return
		case http.MethodPost:
			var req ModeratorProfileRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "INVALID_REQUEST"})
				return
			}
			var role string
			if err := db.QueryRow(`SELECT role FROM accounts WHERE username = $1`, strings.ToLower(req.Username)).Scan(&role); err != nil {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "NOT_FOUND"})
				return
			}
			if viewer.Role == "moderator" && baseRoleFromRaw(role) == "admin" {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "FORBIDDEN"})
				return
			}
			updates := []string{}
			args := []interface{}{}
			argIndex := 1
			username := strings.ToLower(req.Username)

			if strings.TrimSpace(req.DisplayName) != "" {
				displayName := strings.TrimSpace(req.DisplayName)
				if len(displayName) > 32 {
					json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "INVALID_DISPLAY_NAME"})
					return
				}
				updates = append(updates, "display_name = $"+strconv.Itoa(argIndex))
				args = append(args, displayName)
				argIndex++
			}
			if req.Email != "" {
				updates = append(updates, "email = $"+strconv.Itoa(argIndex))
				args = append(args, strings.TrimSpace(req.Email))
				argIndex++
			}
			if req.Bio != "" {
				updates = append(updates, "bio = $"+strconv.Itoa(argIndex))
				args = append(args, strings.TrimSpace(req.Bio))
				argIndex++
			}
			if req.Pronouns != "" {
				updates = append(updates, "pronouns = $"+strconv.Itoa(argIndex))
				args = append(args, strings.TrimSpace(req.Pronouns))
				argIndex++
			}
			if req.Location != "" {
				updates = append(updates, "location = $"+strconv.Itoa(argIndex))
				args = append(args, strings.TrimSpace(req.Location))
				argIndex++
			}
			if req.Website != "" {
				updates = append(updates, "website = $"+strconv.Itoa(argIndex))
				args = append(args, strings.TrimSpace(req.Website))
				argIndex++
			}
			if req.AvatarURL != "" {
				updates = append(updates, "avatar_url = $"+strconv.Itoa(argIndex))
				args = append(args, strings.TrimSpace(req.AvatarURL))
				argIndex++
			}

			if len(updates) == 0 {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "NO_UPDATES"})
				return
			}
			args = append(args, username)
			query := "UPDATE accounts SET " + strings.Join(updates, ", ") + " WHERE username = $" + strconv.Itoa(argIndex)
			_, err := db.Exec(query, args...)
			if err != nil {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: true, Username: username})
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}
}

func adminNotificationsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		adminAccount, ok := requireAdmin(db, w, r)
		if !ok {
			return
		}
		var req AdminNotificationCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		roleInput := strings.TrimSpace(req.RecipientRole)
		if roleInput == "" {
			roleInput = req.TargetRole
		}
		roleInput = strings.ToLower(strings.TrimSpace(roleInput))
		if roleInput == "" {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_TARGET"})
			return
		}
		role := normalizeNotificationRole(roleInput)
		if role != "all" && role != NotificationRolePlayer && role != NotificationRoleModerator && role != NotificationRoleAdmin {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_TARGET"})
			return
		}
		priority := normalizeNotificationPriority(req.Priority)
		if strings.TrimSpace(req.Priority) == "" {
			level := strings.ToLower(strings.TrimSpace(req.Level))
			switch level {
			case "urgent":
				priority = NotificationPriorityCritical
			case "warn":
				priority = NotificationPriorityHigh
			default:
				priority = NotificationPriorityNormal
			}
		}
		category := normalizeNotificationCategory(req.Category)
		if strings.TrimSpace(req.Category) == "" {
			category = NotificationCategoryAdmin
		}
		accountID := strings.TrimSpace(req.RecipientAccountID)
		if accountID == "" {
			accountID = strings.TrimSpace(req.AccountID)
		}
		var expiresAt sql.NullTime
		if strings.TrimSpace(req.ExpiresAt) != "" {
			t, err := time.Parse(time.RFC3339, req.ExpiresAt)
			if err != nil {
				json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_EXPIRES_AT"})
				return
			}
			expiresAt = sql.NullTime{Time: t, Valid: true}
		}
		exp := (*time.Time)(nil)
		if expiresAt.Valid {
			exp = &expiresAt.Time
		}
		targetRoles := []string{role}
		if role == "all" {
			targetRoles = []string{NotificationRolePlayer, NotificationRoleModerator, NotificationRoleAdmin}
		}
		for _, targetRole := range targetRoles {
			if err := createNotification(db, NotificationInput{
				RecipientRole:      targetRole,
				RecipientAccountID: accountID,
				SeasonID:           strings.TrimSpace(req.SeasonID),
				Category:           category,
				Type:               strings.TrimSpace(req.Type),
				Priority:           priority,
				Payload:            req.Payload,
				Message:            strings.TrimSpace(req.Message),
				Link:               strings.TrimSpace(req.Link),
				AckRequired:        req.AckRequired,
				ExpiresAt:          exp,
			}); err != nil {
				json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
		}
		_ = logAdminAction(db, adminAccount.AccountID, "notification_create", "notification", role, "", map[string]interface{}{
			"category":  category,
			"type":      strings.TrimSpace(req.Type),
			"priority":  priority,
			"message":   strings.TrimSpace(req.Message),
			"seasonId":  strings.TrimSpace(req.SeasonID),
			"accountId": accountID,
		})
		json.NewEncoder(w).Encode(SimpleResponse{OK: true})
	}
}

func adminPlayerControlsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		adminAccount, ok := requireAdmin(db, w, r)
		if !ok {
			return
		}
		var req AdminPlayerControlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(AdminPlayerControlResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		playerID := strings.TrimSpace(req.PlayerID)
		if playerID == "" && strings.TrimSpace(req.Username) != "" {
			if err := db.QueryRow(`
				SELECT player_id FROM accounts WHERE username = $1
			`, strings.ToLower(strings.TrimSpace(req.Username))).Scan(&playerID); err != nil {
				json.NewEncoder(w).Encode(AdminPlayerControlResponse{OK: false, Error: "PLAYER_NOT_FOUND"})
				return
			}
		}
		if playerID == "" {
			json.NewEncoder(w).Encode(AdminPlayerControlResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		if req.SetCoins != nil || req.AddCoins != nil || req.SetStars != nil || req.AddStars != nil || req.DripMultiplier != nil || req.DripPaused != nil {
			json.NewEncoder(w).Encode(AdminPlayerControlResponse{OK: false, Error: "ECONOMY_LOCKED"})
			return
		}

		if req.IsBot != nil {
			var currentIsBot bool
			var lastActive time.Time
			var accountID sql.NullString
			if err := db.QueryRow(`
				SELECT p.is_bot, p.last_active_at, a.account_id
				FROM players p
				LEFT JOIN accounts a ON a.player_id = p.player_id
				WHERE p.player_id = $1
			`, playerID).Scan(&currentIsBot, &lastActive, &accountID); err != nil {
				json.NewEncoder(w).Encode(AdminPlayerControlResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if currentIsBot != *req.IsBot {
				if accountID.Valid {
					active, err := isHumanSessionActive(db, accountID.String, lastActive)
					if err != nil {
						json.NewEncoder(w).Encode(AdminPlayerControlResponse{OK: false, Error: "INTERNAL_ERROR"})
						return
					}
					if active {
						json.NewEncoder(w).Encode(AdminPlayerControlResponse{OK: false, Error: "HUMAN_ACTIVE"})
						return
					}
				}
				_, err := db.Exec(`
					UPDATE players
					SET is_bot = $2
					WHERE player_id = $1
				`, playerID, *req.IsBot)
				if err != nil {
					json.NewEncoder(w).Encode(AdminPlayerControlResponse{OK: false, Error: "INTERNAL_ERROR"})
					return
				}
				_ = logAdminBotToggle(db, adminAccount.AccountID, playerID, currentIsBot, *req.IsBot, nil)
				_ = logAdminAction(db, adminAccount.AccountID, "bot_toggle", "player", playerID, "", map[string]interface{}{
					"wasBot": currentIsBot,
					"nowBot": *req.IsBot,
				})
			}
		}
		if req.BotProfile != nil {
			profile := strings.TrimSpace(*req.BotProfile)
			_, err := db.Exec(`
				UPDATE players
				SET bot_profile = NULLIF($2, '')
				WHERE player_id = $1
			`, playerID, profile)
			if err != nil {
				json.NewEncoder(w).Encode(AdminPlayerControlResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
		}
		if req.TouchActive {
			_, err := db.Exec(`
				UPDATE players
				SET last_active_at = NOW()
				WHERE player_id = $1
			`, playerID)
			if err != nil {
				json.NewEncoder(w).Encode(AdminPlayerControlResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
		}

		var coins int64
		var stars int64
		var dripMultiplier float64
		var dripPaused bool
		var lastActive time.Time
		var lastGrant time.Time
		var isBot bool
		var botProfile sql.NullString
		if err := db.QueryRow(`
			SELECT coins, stars, drip_multiplier, drip_paused, last_active_at, last_coin_grant_at, is_bot, bot_profile
			FROM players
			WHERE player_id = $1
		`, playerID).Scan(&coins, &stars, &dripMultiplier, &dripPaused, &lastActive, &lastGrant, &isBot, &botProfile); err != nil {
			json.NewEncoder(w).Encode(AdminPlayerControlResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		json.NewEncoder(w).Encode(AdminPlayerControlResponse{
			OK:             true,
			PlayerID:       playerID,
			Coins:          coins,
			Stars:          stars,
			DripMultiplier: dripMultiplier,
			DripPaused:     dripPaused,
			IsBot:          isBot,
			BotProfile:     botProfile.String,
			LastActiveAt:   lastActive,
			LastGrantAt:    lastGrant,
		})
	}
}

func adminSettingsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(AdminGlobalSettingsResponse{OK: true, Settings: GetGlobalSettings()})
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}
}

func adminStarPurchaseLogHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		query := r.URL.Query()
		limit := 50
		if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 250 {
			limit = 250
		}

		accountID := strings.TrimSpace(query.Get("accountId"))
		playerID := strings.TrimSpace(query.Get("playerId"))
		seasonID := strings.TrimSpace(query.Get("seasonId"))
		purchaseType := strings.TrimSpace(query.Get("purchaseType"))
		variant := strings.TrimSpace(query.Get("variant"))
		fromRaw := strings.TrimSpace(query.Get("from"))
		toRaw := strings.TrimSpace(query.Get("to"))

		parseInt64 := func(key string) (*int64, bool) {
			raw := strings.TrimSpace(query.Get(key))
			if raw == "" {
				return nil, false
			}
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return nil, false
			}
			return &value, true
		}

		parseTime := func(raw string) (*time.Time, bool) {
			if raw == "" {
				return nil, false
			}
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return nil, false
			}
			return &parsed, true
		}

		fromTime, hasFrom := parseTime(fromRaw)
		toTime, hasTo := parseTime(toRaw)
		minPrice, hasMinPrice := parseInt64("minPrice")
		maxPrice, hasMaxPrice := parseInt64("maxPrice")
		minCoinsBefore, hasMinCoinsBefore := parseInt64("minCoinsBefore")
		maxCoinsBefore, hasMaxCoinsBefore := parseInt64("maxCoinsBefore")
		minCoinsAfter, hasMinCoinsAfter := parseInt64("minCoinsAfter")
		maxCoinsAfter, hasMaxCoinsAfter := parseInt64("maxCoinsAfter")
		minStarsBefore, hasMinStarsBefore := parseInt64("minStarsBefore")
		maxStarsBefore, hasMaxStarsBefore := parseInt64("maxStarsBefore")
		minStarsAfter, hasMinStarsAfter := parseInt64("minStarsAfter")
		maxStarsAfter, hasMaxStarsAfter := parseInt64("maxStarsAfter")

		clauses := []string{}
		args := []interface{}{}
		argIndex := 1
		if accountID != "" {
			clauses = append(clauses, "account_id = $"+strconv.Itoa(argIndex))
			args = append(args, accountID)
			argIndex++
		}
		if playerID != "" {
			clauses = append(clauses, "player_id = $"+strconv.Itoa(argIndex))
			args = append(args, playerID)
			argIndex++
		}
		if seasonID != "" {
			clauses = append(clauses, "season_id = $"+strconv.Itoa(argIndex))
			args = append(args, seasonID)
			argIndex++
		}
		if purchaseType != "" {
			clauses = append(clauses, "purchase_type = $"+strconv.Itoa(argIndex))
			args = append(args, purchaseType)
			argIndex++
		}
		if variant != "" {
			clauses = append(clauses, "variant = $"+strconv.Itoa(argIndex))
			args = append(args, variant)
			argIndex++
		}
		if hasFrom {
			clauses = append(clauses, "created_at >= $"+strconv.Itoa(argIndex))
			args = append(args, *fromTime)
			argIndex++
		}
		if hasTo {
			clauses = append(clauses, "created_at <= $"+strconv.Itoa(argIndex))
			args = append(args, *toTime)
			argIndex++
		}
		if hasMinPrice {
			clauses = append(clauses, "price_paid >= $"+strconv.Itoa(argIndex))
			args = append(args, *minPrice)
			argIndex++
		}
		if hasMaxPrice {
			clauses = append(clauses, "price_paid <= $"+strconv.Itoa(argIndex))
			args = append(args, *maxPrice)
			argIndex++
		}
		if hasMinCoinsBefore {
			clauses = append(clauses, "coins_before >= $"+strconv.Itoa(argIndex))
			args = append(args, *minCoinsBefore)
			argIndex++
		}
		if hasMaxCoinsBefore {
			clauses = append(clauses, "coins_before <= $"+strconv.Itoa(argIndex))
			args = append(args, *maxCoinsBefore)
			argIndex++
		}
		if hasMinCoinsAfter {
			clauses = append(clauses, "coins_after >= $"+strconv.Itoa(argIndex))
			args = append(args, *minCoinsAfter)
			argIndex++
		}
		if hasMaxCoinsAfter {
			clauses = append(clauses, "coins_after <= $"+strconv.Itoa(argIndex))
			args = append(args, *maxCoinsAfter)
			argIndex++
		}
		if hasMinStarsBefore {
			clauses = append(clauses, "stars_before >= $"+strconv.Itoa(argIndex))
			args = append(args, *minStarsBefore)
			argIndex++
		}
		if hasMaxStarsBefore {
			clauses = append(clauses, "stars_before <= $"+strconv.Itoa(argIndex))
			args = append(args, *maxStarsBefore)
			argIndex++
		}
		if hasMinStarsAfter {
			clauses = append(clauses, "stars_after >= $"+strconv.Itoa(argIndex))
			args = append(args, *minStarsAfter)
			argIndex++
		}
		if hasMaxStarsAfter {
			clauses = append(clauses, "stars_after <= $"+strconv.Itoa(argIndex))
			args = append(args, *maxStarsAfter)
			argIndex++
		}

		sqlQuery := `
			SELECT id, account_id, player_id, season_id, purchase_type, variant,
				price_paid, coins_before, coins_after, stars_before, stars_after, created_at
			FROM star_purchase_log
		`
		if len(clauses) > 0 {
			sqlQuery += " WHERE " + strings.Join(clauses, " AND ")
		}
		sqlQuery += " ORDER BY created_at DESC LIMIT " + strconv.Itoa(limit)

		rows, err := db.Query(sqlQuery, args...)
		if err != nil {
			json.NewEncoder(w).Encode(AdminStarPurchaseLogResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer rows.Close()

		items := []AdminStarPurchaseLogItem{}
		for rows.Next() {
			var item AdminStarPurchaseLogItem
			if err := rows.Scan(
				&item.ID,
				&item.AccountID,
				&item.PlayerID,
				&item.SeasonID,
				&item.PurchaseType,
				&item.Variant,
				&item.PricePaid,
				&item.CoinsBefore,
				&item.CoinsAfter,
				&item.StarsBefore,
				&item.StarsAfter,
				&item.CreatedAt,
			); err != nil {
				continue
			}
			items = append(items, item)
		}

		json.NewEncoder(w).Encode(AdminStarPurchaseLogResponse{OK: true, Items: items})
	}
}

func adminBotListHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}

		rows, err := db.Query(`
			SELECT p.player_id, a.username, a.display_name, p.is_bot, p.bot_profile
			FROM players p
			LEFT JOIN accounts a ON a.player_id = p.player_id
			WHERE p.is_bot = TRUE
			ORDER BY a.username ASC
		`)
		if err != nil {
			json.NewEncoder(w).Encode(AdminBotListResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer rows.Close()

		bots := []AdminBotListItem{}
		for rows.Next() {
			var item AdminBotListItem
			var botProfile sql.NullString
			if err := rows.Scan(&item.PlayerID, &item.Username, &item.DisplayName, &item.IsBot, &botProfile); err != nil {
				continue
			}
			if botProfile.Valid {
				item.BotProfile = botProfile.String
			}
			bots = append(bots, item)
		}

		json.NewEncoder(w).Encode(AdminBotListResponse{OK: true, Bots: bots})
	}
}

func adminBotCreateHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		adminAccount, ok := requireAdmin(db, w, r)
		if !ok {
			return
		}

		var req AdminBotCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(AdminBotCreateResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		username := strings.TrimSpace(req.Username)
		if username == "" {
			json.NewEncoder(w).Encode(AdminBotCreateResponse{OK: false, Error: "INVALID_USERNAME"})
			return
		}
		password := strings.TrimSpace(req.Password)
		if password == "" {
			generated, err := randomToken(9)
			if err != nil {
				json.NewEncoder(w).Encode(AdminBotCreateResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			password = generated
		}

		account, err := createAccount(db, username, password, req.DisplayName, "")
		if err != nil {
			json.NewEncoder(w).Encode(AdminBotCreateResponse{OK: false, Error: "CREATE_FAILED"})
			return
		}

		if _, err := LoadOrCreatePlayer(db, account.PlayerID); err != nil {
			_, _ = db.Exec(`DELETE FROM accounts WHERE account_id = $1`, account.AccountID)
			json.NewEncoder(w).Encode(AdminBotCreateResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		if _, err := db.Exec(`
			UPDATE players
			SET is_bot = TRUE, bot_profile = $2
			WHERE player_id = $1
		`, account.PlayerID, strings.TrimSpace(req.BotProfile)); err != nil {
			_, _ = db.Exec(`DELETE FROM players WHERE player_id = $1`, account.PlayerID)
			_, _ = db.Exec(`DELETE FROM accounts WHERE account_id = $1`, account.AccountID)
			json.NewEncoder(w).Encode(AdminBotCreateResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		json.NewEncoder(w).Encode(AdminBotCreateResponse{
			OK:          true,
			PlayerID:    account.PlayerID,
			Username:    account.Username,
			DisplayName: account.DisplayName,
			Password:    password,
		})
		_ = logAdminAction(db, adminAccount.AccountID, "bot_create", "player", account.PlayerID, "", map[string]interface{}{
			"username":    account.Username,
			"displayName": account.DisplayName,
		})
	}
}

func adminBotDeleteHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		adminAccount, ok := requireAdmin(db, w, r)
		if !ok {
			return
		}

		var req AdminBotDeleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		playerID := strings.TrimSpace(req.PlayerID)
		username := strings.TrimSpace(strings.ToLower(req.Username))
		if playerID == "" && username == "" {
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		var accountID sql.NullString
		var resolvedPlayerID string
		var isBot bool
		err := db.QueryRow(`
			SELECT p.player_id, a.account_id, p.is_bot
			FROM players p
			LEFT JOIN accounts a ON a.player_id = p.player_id
			WHERE p.player_id = $1 OR a.username = $2
			LIMIT 1
		`, playerID, username).Scan(&resolvedPlayerID, &accountID, &isBot)
		if err == sql.ErrNoRows {
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "NOT_FOUND"})
			return
		}
		if err != nil {
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !isBot {
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "NOT_BOT"})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if accountID.Valid {
			if _, err := tx.Exec(`DELETE FROM sessions WHERE account_id = $1`, accountID.String); err != nil {
				tx.Rollback()
				json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if _, err := tx.Exec(`DELETE FROM notification_reads WHERE account_id = $1`, accountID.String); err != nil {
				tx.Rollback()
				json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if _, err := tx.Exec(`DELETE FROM accounts WHERE account_id = $1`, accountID.String); err != nil {
				tx.Rollback()
				json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
		}
		if _, err := tx.Exec(`DELETE FROM player_boosts WHERE player_id = $1`, resolvedPlayerID); err != nil {
			tx.Rollback()
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if _, err := tx.Exec(`DELETE FROM player_star_variants WHERE player_id = $1`, resolvedPlayerID); err != nil {
			tx.Rollback()
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if _, err := tx.Exec(`DELETE FROM player_faucet_claims WHERE player_id = $1`, resolvedPlayerID); err != nil {
			tx.Rollback()
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if _, err := tx.Exec(`DELETE FROM player_ip_associations WHERE player_id = $1`, resolvedPlayerID); err != nil {
			tx.Rollback()
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if _, err := tx.Exec(`DELETE FROM players WHERE player_id = $1`, resolvedPlayerID); err != nil {
			tx.Rollback()
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if err := tx.Commit(); err != nil {
			json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		json.NewEncoder(w).Encode(AdminBotDeleteResponse{OK: true, PlayerID: resolvedPlayerID})
		_ = logAdminAction(db, adminAccount.AccountID, "bot_delete", "player", resolvedPlayerID, "", map[string]interface{}{
			"accountId": accountID.String,
			"username":  username,
		})
	}
}

func adminProfileActionHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		adminAccount, ok := requireAdmin(db, w, r)
		if !ok {
			return
		}
		var req AdminProfileActionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Username) == "" {
			json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		username := strings.ToLower(strings.TrimSpace(req.Username))
		if username == strings.ToLower(adminAccount.Username) {
			json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "CANNOT_TARGET_SELF"})
			return
		}
		action := strings.ToLower(strings.TrimSpace(req.Action))
		if action != "freeze" && action != "unfreeze" && action != "delete" {
			json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INVALID_ACTION"})
			return
		}

		var accountID string
		var playerID string
		var role string
		if err := db.QueryRow(`
			SELECT account_id, player_id, role
			FROM accounts
			WHERE username = $1
		`, username).Scan(&accountID, &playerID, &role); err != nil {
			json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "NOT_FOUND"})
			return
		}

		switch action {
		case "freeze":
			if isFrozenRole(role) {
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "ALREADY_FROZEN"})
				return
			}
			frozenRole := "frozen:" + baseRoleFromRaw(role)
			if _, err := db.Exec(`UPDATE accounts SET role = $2 WHERE account_id = $1`, accountID, frozenRole); err != nil {
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			_, _ = db.Exec(`DELETE FROM sessions WHERE account_id = $1`, accountID)
			emitNotification(db, NotificationInput{
				RecipientRole: NotificationRoleAdmin,
				Category:      NotificationCategoryAdmin,
				Type:          "profile_frozen",
				Priority:      NotificationPriorityHigh,
				Message:       "Profile frozen: @" + username,
				Payload: map[string]interface{}{
					"username": username,
					"action":   "freeze",
				},
			})
			_ = logAdminAction(db, adminAccount.AccountID, "profile_freeze", "account", accountID, "", map[string]interface{}{
				"username":     username,
				"playerId":     playerID,
				"previousRole": role,
			})
			json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: true, Username: username, Role: baseRoleFromRaw(frozenRole), Frozen: true})
			return
		case "unfreeze":
			if !isFrozenRole(role) {
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "NOT_FROZEN"})
				return
			}
			baseRole := baseRoleFromRaw(role)
			if _, err := db.Exec(`UPDATE accounts SET role = $2 WHERE account_id = $1`, accountID, baseRole); err != nil {
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			emitNotification(db, NotificationInput{
				RecipientRole: NotificationRoleAdmin,
				Category:      NotificationCategoryAdmin,
				Type:          "profile_unfrozen",
				Priority:      NotificationPriorityNormal,
				Message:       "Profile unfrozen: @" + username,
				Payload: map[string]interface{}{
					"username": username,
					"action":   "unfreeze",
				},
			})
			_ = logAdminAction(db, adminAccount.AccountID, "profile_unfreeze", "account", accountID, "", map[string]interface{}{
				"username":     username,
				"playerId":     playerID,
				"previousRole": role,
			})
			json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: true, Username: username, Role: baseRole, Frozen: false})
			return
		case "delete":
			tx, err := db.Begin()
			if err != nil {
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if _, err := tx.Exec(`DELETE FROM sessions WHERE account_id = $1`, accountID); err != nil {
				tx.Rollback()
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if _, err := tx.Exec(`DELETE FROM notification_reads WHERE account_id = $1`, accountID); err != nil {
				tx.Rollback()
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if _, err := tx.Exec(`DELETE FROM accounts WHERE account_id = $1`, accountID); err != nil {
				tx.Rollback()
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if _, err := tx.Exec(`DELETE FROM player_boosts WHERE player_id = $1`, playerID); err != nil {
				tx.Rollback()
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if _, err := tx.Exec(`DELETE FROM player_star_variants WHERE player_id = $1`, playerID); err != nil {
				tx.Rollback()
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if _, err := tx.Exec(`DELETE FROM player_faucet_claims WHERE player_id = $1`, playerID); err != nil {
				tx.Rollback()
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if _, err := tx.Exec(`DELETE FROM player_ip_associations WHERE player_id = $1`, playerID); err != nil {
				tx.Rollback()
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if _, err := tx.Exec(`DELETE FROM players WHERE player_id = $1`, playerID); err != nil {
				tx.Rollback()
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			if err := tx.Commit(); err != nil {
				json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			emitNotification(db, NotificationInput{
				RecipientRole: NotificationRoleAdmin,
				Category:      NotificationCategoryAdmin,
				Type:          "profile_deleted",
				Priority:      NotificationPriorityHigh,
				Message:       "Profile deleted: @" + username,
				Payload: map[string]interface{}{
					"username": username,
					"action":   "delete",
				},
			})
			_ = logAdminAction(db, adminAccount.AccountID, "profile_delete", "account", accountID, "", map[string]interface{}{
				"username":     username,
				"playerId":     playerID,
				"previousRole": role,
			})
			json.NewEncoder(w).Encode(AdminProfileActionResponse{OK: true, Username: username})
			return
		}
	}
}

func isHumanSessionActive(db *sql.DB, accountID string, lastActive time.Time) (bool, error) {
	windowSeconds := 600
	if raw := strings.TrimSpace(os.Getenv("BOT_TOGGLE_ACTIVE_WINDOW_SECONDS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			windowSeconds = parsed
		}
	}
	if time.Since(lastActive) > time.Duration(windowSeconds)*time.Second {
		return false, nil
	}
	var count int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM sessions
		WHERE account_id = $1 AND expires_at > NOW()
	`, accountID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func parseSeasonControlsPath(path string) (string, bool) {
	const prefix = "/admin/seasons/"
	const suffix = "/controls"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	value := strings.TrimPrefix(path, prefix)
	value = strings.TrimSuffix(value, suffix)
	value = strings.Trim(value, "/")
	if value == "" {
		return "", false
	}
	return value, true
}

func normalizeUUIDFromString(input string, namespace string) string {
	value := strings.TrimSpace(input)
	if value == "" {
		return ""
	}
	if isUUIDString(value) {
		return strings.ToLower(value)
	}
	return deriveUUIDFromString(namespace, value)
}

func isUUIDString(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') {
				continue
			}
			return false
		}
	}
	return true
}

func deriveUUIDFromString(namespace string, value string) string {
	sum := sha1.Sum([]byte(namespace + ":" + value))
	bytes := make([]byte, 16)
	copy(bytes, sum[:16])
	bytes[6] = (bytes[6] & 0x0f) | 0x50
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func generateRandomUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func logAdminAction(db *sql.DB, adminAccountID string, actionType string, scopeType string, scopeID string, reason string, details map[string]interface{}) error {
	if strings.TrimSpace(adminAccountID) == "" || strings.TrimSpace(actionType) == "" || strings.TrimSpace(scopeType) == "" || strings.TrimSpace(scopeID) == "" {
		return nil
	}
	var payload interface{}
	if details != nil {
		bytes, err := json.Marshal(details)
		if err != nil {
			return err
		}
		payload = string(bytes)
	}
	_, err := db.Exec(`
		INSERT INTO admin_audit_log (admin_account_id, action_type, scope_type, scope_id, reason, details, created_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, NOW())
	`, adminAccountID, actionType, scopeType, scopeID, reason, payload)
	return err
}

func logAdminBotToggle(db *sql.DB, adminAccountID string, playerID string, wasBot bool, nowBot bool, botProfile *string) error {
	payload := map[string]interface{}{
		"adminAccountId": adminAccountID,
		"playerId":       playerID,
		"wasBot":         wasBot,
		"nowBot":         nowBot,
	}
	if botProfile != nil {
		payload["botProfile"] = *botProfile
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO player_telemetry (account_id, player_id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, adminAccountID, playerID, "admin_bot_toggle", bytes)
	return err
}
