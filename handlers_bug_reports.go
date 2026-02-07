package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// bugReportSubmitHandler is the player-facing POST /bugs/report endpoint
// Accepts authenticated and anonymous submissions
func bugReportSubmitHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Check rate limit using existing auth throttling
		var account *Account
		var playerID *string

		// Optional auth: if session exists, use it; otherwise allow anonymous
		acct, _, err := getSessionAccount(db, r)
		if err == nil && acct != nil {
			account = acct
			playerID = &acct.PlayerID
		}

		ip := getClientIP(r)
		limit, window := authRateLimitConfig("bug_report")
		allowedRate, retryAfter, err := checkAuthRateLimit(db, ip, "bug_report", limit, window)
		if err != nil {
			log.Println("bug report: rate limit error:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(CreateBugReportResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if !allowedRate {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(CreateBugReportResponse{OK: false, Error: "RATE_LIMIT"})
			return
		}

		// Parse request
		var req CreateBugReportRequest
		err = json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(CreateBugReportResponse{
				OK:    false,
				Error: "INVALID_REQUEST",
			})
			return
		}

		// Validate input
		if req.Title == "" || req.Description == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(CreateBugReportResponse{
				OK:    false,
				Error: "MISSING_REQUIRED_FIELDS",
			})
			return
		}

		// Enforce reasonable limits
		if len(req.Title) > 200 {
			req.Title = req.Title[:200]
		}
		if len(req.Description) > 5000 {
			req.Description = req.Description[:5000]
		}
		if req.ClientVersion != "" && len(req.ClientVersion) > 50 {
			req.ClientVersion = req.ClientVersion[:50]
		}

		// Get current season ID
		seasonID := currentSeasonID()

		// Submit bug report (append-only insert)
		var clientVersion *string
		if req.ClientVersion != "" {
			clientVersion = &req.ClientVersion
		}

		reportID, err := SubmitBugReport(db, playerID, seasonID, req.Title, req.Description, req.Category, clientVersion)
		if err != nil {
			log.Printf("bug report submission failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(CreateBugReportResponse{
				OK:    false,
				Error: "SUBMISSION_FAILED",
			})
			return
		}

		// Log event for audit trail
		if account != nil {
			emitServerTelemetry(db, &account.AccountID, account.PlayerID, "bug_report_submitted", map[string]interface{}{
				"report_id": reportID,
				"category":  req.Category,
				"title":     req.Title[:min(30, len(req.Title))], // Log first 30 chars for brevity
			})
		}

		// Success response (no feedback loop about the report)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(CreateBugReportResponse{OK: true})
	}
}

// adminBugReportsHandler is the read-only GET /admin/bugs endpoint
// Admins only. Returns paginated bug report list.
func adminBugReportsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Require admin auth
		_, ok := requireAdmin(db, w, r)
		if !ok {
			return
		}

		// Parse pagination params
		limitStr := r.URL.Query().Get("limit")
		limit := 50
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}
		if limit > 500 {
			limit = 500
		}

		offsetStr := r.URL.Query().Get("offset")
		offset := 0
		if offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		// Get current season
		seasonID := currentSeasonID()

		// Fetch bug reports
		reports, total, err := GetBugReports(db, seasonID, limit, offset)
		if err != nil {
			log.Printf("admin bug reports fetch failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AdminBugReportsResponse{
				OK:    false,
				Error: "FETCH_FAILED",
			})
			return
		}

		// Convert to admin response format
		items := make([]AdminBugReport, len(reports))
		for i, report := range reports {
			items[i] = report
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AdminBugReportsResponse{
			OK:     true,
			Items:  items,
			Total:  total,
			Limit:  limit,
			Offset: offset,
		})
	}
}

// Helper to compute min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
