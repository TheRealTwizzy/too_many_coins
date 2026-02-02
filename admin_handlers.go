package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

type AdminTelemetrySeriesPoint struct {
	Bucket    time.Time `json:"bucket"`
	EventType string    `json:"eventType"`
	Count     int       `json:"count"`
}

type AdminTelemetryResponse struct {
	OK       bool                        `json:"ok"`
	Error    string                      `json:"error,omitempty"`
	Series   []AdminTelemetrySeriesPoint `json:"series,omitempty"`
	Feedback []AdminFeedbackItem         `json:"feedback,omitempty"`
}

type AdminFeedbackItem struct {
	Rating    *int      `json:"rating,omitempty"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type AdminEconomyResponse struct {
	OK                  bool   `json:"ok"`
	Error               string `json:"error,omitempty"`
	DailyEmissionTarget int    `json:"dailyEmissionTarget,omitempty"`
	FaucetsEnabled      bool   `json:"faucetsEnabled,omitempty"`
	SinksEnabled        bool   `json:"sinksEnabled,omitempty"`
	TelemetryEnabled    bool   `json:"telemetryEnabled,omitempty"`
}

type AdminEconomyUpdateRequest struct {
	DailyEmissionTarget *int  `json:"dailyEmissionTarget,omitempty"`
	FaucetsEnabled      *bool `json:"faucetsEnabled,omitempty"`
	SinksEnabled        *bool `json:"sinksEnabled,omitempty"`
	TelemetryEnabled    *bool `json:"telemetryEnabled,omitempty"`
}

func requireAdmin(db *sql.DB, w http.ResponseWriter, r *http.Request) (*Account, bool) {
	account, _, err := getSessionAccount(db, r)
	if err != nil || account == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, false
	}
	if account.Role != "admin" || account.AdminKeyHash == "" {
		w.WriteHeader(http.StatusForbidden)
		return nil, false
	}
	provided := r.Header.Get("X-Admin-Key")
	if provided == "" || !verifyAdminKey(account.AdminKeyHash, provided) {
		w.WriteHeader(http.StatusUnauthorized)
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
	if (account.Role != "admin" && account.Role != "moderator") || account.AdminKeyHash == "" {
		w.WriteHeader(http.StatusForbidden)
		return nil, false
	}
	provided := r.Header.Get("X-Admin-Key")
	if provided == "" || !verifyAdminKey(account.AdminKeyHash, provided) {
		w.WriteHeader(http.StatusUnauthorized)
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

		feedbackRows, err := db.Query(`
			SELECT rating, message, created_at
			FROM player_feedback
			ORDER BY created_at DESC
			LIMIT 25
		`)
		if err != nil {
			json.NewEncoder(w).Encode(AdminTelemetryResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer feedbackRows.Close()

		feedback := []AdminFeedbackItem{}
		for feedbackRows.Next() {
			var rating sql.NullInt64
			var message string
			var created time.Time
			if err := feedbackRows.Scan(&rating, &message, &created); err != nil {
				continue
			}
			item := AdminFeedbackItem{
				Message:   message,
				CreatedAt: created,
			}
			if rating.Valid {
				value := int(rating.Int64)
				item.Rating = &value
			}
			feedback = append(feedback, item)
		}

		json.NewEncoder(w).Encode(AdminTelemetryResponse{
			OK:       true,
			Series:   series,
			Feedback: feedback,
		})
	}
}

func adminEconomyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}

		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(AdminEconomyResponse{
				OK:                  true,
				DailyEmissionTarget: economy.DailyEmissionTarget(),
				FaucetsEnabled:      featureFlags.FaucetsEnabled,
				SinksEnabled:        featureFlags.SinksEnabled,
				TelemetryEnabled:    featureFlags.Telemetry,
			})
			return
		case http.MethodPost:
			var req AdminEconomyUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				json.NewEncoder(w).Encode(AdminEconomyResponse{OK: false, Error: "INVALID_REQUEST"})
				return
			}
			if req.DailyEmissionTarget != nil {
				value := *req.DailyEmissionTarget
				if value < 0 || value > 10000 {
					json.NewEncoder(w).Encode(AdminEconomyResponse{OK: false, Error: "INVALID_EMISSION_TARGET"})
					return
				}
				economy.SetDailyEmissionTarget(value)
			}
			if req.FaucetsEnabled != nil {
				featureFlags.FaucetsEnabled = *req.FaucetsEnabled
			}
			if req.SinksEnabled != nil {
				featureFlags.SinksEnabled = *req.SinksEnabled
			}
			if req.TelemetryEnabled != nil {
				featureFlags.Telemetry = *req.TelemetryEnabled
			}

			json.NewEncoder(w).Encode(AdminEconomyResponse{
				OK:                  true,
				DailyEmissionTarget: economy.DailyEmissionTarget(),
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

type AdminKeySetRequest struct {
	AdminKey string `json:"adminKey"`
}

type AdminKeySetResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func adminKeySetHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		setupKey := os.Getenv("ADMIN_SETUP_KEY")
		if setupKey == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		provided := r.Header.Get("X-Admin-Setup")
		if provided != setupKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		account, _, err := getSessionAccount(db, r)
		if err != nil || account == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req AdminKeySetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AdminKey == "" {
			json.NewEncoder(w).Encode(AdminKeySetResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		if len(req.AdminKey) < 8 {
			json.NewEncoder(w).Encode(AdminKeySetResponse{OK: false, Error: "WEAK_KEY"})
			return
		}
		if err := setAdminKey(db, account.AccountID, req.AdminKey); err != nil {
			json.NewEncoder(w).Encode(AdminKeySetResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		_ = setAccountRole(db, account.AccountID, "admin")
		json.NewEncoder(w).Encode(AdminKeySetResponse{OK: true})
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
		if _, ok := requireAdmin(db, w, r); !ok {
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
		json.NewEncoder(w).Encode(AdminRoleResponse{OK: true})
	}
}

type AdminKeyForUserRequest struct {
	Username string `json:"username"`
	AdminKey string `json:"adminKey"`
}

type AdminKeyForUserResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func adminKeyForUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if _, ok := requireAdmin(db, w, r); !ok {
			return
		}
		var req AdminKeyForUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.AdminKey == "" {
			json.NewEncoder(w).Encode(AdminKeyForUserResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}
		if len(req.AdminKey) < 8 {
			json.NewEncoder(w).Encode(AdminKeyForUserResponse{OK: false, Error: "WEAK_KEY"})
			return
		}
		if err := setAdminKeyByUsername(db, req.Username, req.AdminKey); err != nil {
			json.NewEncoder(w).Encode(AdminKeyForUserResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		json.NewEncoder(w).Encode(AdminKeyForUserResponse{OK: true})
	}
}

type ModeratorProfileRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type ModeratorProfileResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

func moderatorProfileHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireModerator(db, w, r); !ok {
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
			err := db.QueryRow(`
				SELECT display_name
				FROM accounts
				WHERE username = $1
			`, strings.ToLower(username)).Scan(&displayName)
			if err == sql.ErrNoRows {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "NOT_FOUND"})
				return
			}
			if err != nil {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: true, Username: strings.ToLower(username), DisplayName: displayName})
			return
		case http.MethodPost:
			var req ModeratorProfileRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "INVALID_REQUEST"})
				return
			}
			displayName := strings.TrimSpace(req.DisplayName)
			if displayName == "" || len(displayName) > 32 {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "INVALID_DISPLAY_NAME"})
				return
			}
			_, err := db.Exec(`
				UPDATE accounts
				SET display_name = $2
				WHERE username = $1
			`, strings.ToLower(req.Username), displayName)
			if err != nil {
				json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: false, Error: "INTERNAL_ERROR"})
				return
			}
			json.NewEncoder(w).Encode(ModeratorProfileResponse{OK: true, Username: strings.ToLower(req.Username), DisplayName: displayName})
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}
}
