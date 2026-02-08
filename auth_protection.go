package main

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"time"
)

func authRateLimitConfig(action string) (int, time.Duration) {
	switch action {
	case "signup":
		limit := parseEnvInt("SIGNUP_RATE_LIMIT", 5)
		windowSeconds := parseEnvInt("SIGNUP_RATE_WINDOW_SECONDS", 600)
		return limit, time.Duration(windowSeconds) * time.Second
	case "login":
		limit := parseEnvInt("LOGIN_RATE_LIMIT", 12)
		windowSeconds := parseEnvInt("LOGIN_RATE_WINDOW_SECONDS", 600)
		return limit, time.Duration(windowSeconds) * time.Second
	default:
		limit := parseEnvInt("AUTH_RATE_LIMIT", 10)
		windowSeconds := parseEnvInt("AUTH_RATE_WINDOW_SECONDS", 600)
		return limit, time.Duration(windowSeconds) * time.Second
	}
}

func checkAuthRateLimit(db *sql.DB, ip string, action string, limit int, window time.Duration) (bool, int, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" || limit <= 0 || window <= 0 {
		return true, 0, nil
	}

	now := time.Now().UTC()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()

	var windowStart time.Time
	var attempts int
	err = tx.QueryRow(`
		SELECT window_start, attempt_count
		FROM auth_rate_limits
		WHERE ip = $1 AND action = $2
		FOR UPDATE
	`, ip, action).Scan(&windowStart, &attempts)
	if err == sql.ErrNoRows {
		_, err = tx.Exec(`
			INSERT INTO auth_rate_limits (ip, action, window_start, attempt_count, updated_at)
			VALUES ($1, $2, $3, 1, $3)
		`, ip, action, now)
		if err != nil {
			return false, 0, err
		}
		if err := tx.Commit(); err != nil {
			return false, 0, err
		}
		return true, 0, nil
	}
	if err != nil {
		return false, 0, err
	}

	elapsed := now.Sub(windowStart)
	if elapsed >= window {
		_, err = tx.Exec(`
			UPDATE auth_rate_limits
			SET window_start = $3,
				attempt_count = 1,
				updated_at = $3
			WHERE ip = $1 AND action = $2
		`, ip, action, now)
		if err != nil {
			return false, 0, err
		}
		if err := tx.Commit(); err != nil {
			return false, 0, err
		}
		return true, 0, nil
	}

	if attempts >= limit {
		retryAfter := int(window.Seconds() - elapsed.Seconds())
		if retryAfter < 0 {
			retryAfter = 0
		}
		_, _ = tx.Exec(`
			UPDATE auth_rate_limits
			SET updated_at = $3
			WHERE ip = $1 AND action = $2
		`, ip, action, now)
		if err := tx.Commit(); err != nil {
			return false, 0, err
		}
		return false, retryAfter, nil
	}

	_, err = tx.Exec(`
		UPDATE auth_rate_limits
		SET attempt_count = attempt_count + 1,
			updated_at = $3
		WHERE ip = $1 AND action = $2
	`, ip, action, now)
	if err != nil {
		return false, 0, err
	}
	if err := tx.Commit(); err != nil {
		return false, 0, err
	}
	return true, 0, nil
}

func parseEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

type AccountAgeScaling struct {
	EarnMultiplier     float64
	CooldownMultiplier float64
	MaxBulkMultiplier  float64
}

func accountAgeScaling(db *sql.DB, accountID string, now time.Time) (AccountAgeScaling, error) {
	// Account age is a soft signal only. Hard gating first-session play is forbidden.
	scaling := AccountAgeScaling{
		EarnMultiplier:     1.0,
		CooldownMultiplier: 1.0,
		MaxBulkMultiplier:  1.0,
	}

	softWindowMinutes := parseEnvInt("NEW_ACCOUNT_COOLDOWN_MINUTES", 30)
	if softWindowMinutes <= 0 {
		return scaling, nil
	}

	var createdAt time.Time
	if err := db.QueryRow(`
		SELECT created_at
		FROM accounts
		WHERE account_id = $1
	`, accountID).Scan(&createdAt); err != nil {
		return scaling, err
	}

	window := time.Duration(softWindowMinutes) * time.Minute
	elapsed := now.Sub(createdAt)
	if elapsed <= 0 {
		elapsed = 0
	}
	if elapsed >= window {
		return scaling, nil
	}

	progress := elapsed.Seconds() / window.Seconds()
	if progress < 0 {
		progress = 0
	} else if progress > 1 {
		progress = 1
	}

	scaling.EarnMultiplier = 0.7 + (0.3 * progress)
	scaling.CooldownMultiplier = 1.6 - (0.6 * progress)
	scaling.MaxBulkMultiplier = 0.25 + (0.75 * progress)

	if scaling.EarnMultiplier < 0.5 {
		scaling.EarnMultiplier = 0.5
	}
	if scaling.CooldownMultiplier < 1.0 {
		scaling.CooldownMultiplier = 1.0
	}
	if scaling.MaxBulkMultiplier < 0.2 {
		scaling.MaxBulkMultiplier = 0.2
	}

	return scaling, nil
}
