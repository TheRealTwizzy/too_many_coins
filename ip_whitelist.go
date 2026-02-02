package main

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

func getWhitelistMaxForIP(db *sql.DB, ip string) (int, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return 0, nil
	}
	var max int
	err := db.QueryRow(`
		SELECT max_accounts
		FROM ip_whitelist
		WHERE ip = $1
	`, ip).Scan(&max)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return max, nil
}

func CanSignupFromIP(db *sql.DB, ip string) (bool, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return true, nil
	}
	count, err := countPlayersForIP(db, ip)
	if err != nil {
		return false, err
	}
	maxAllowed, err := getWhitelistMaxForIP(db, ip)
	if err != nil {
		return false, err
	}
	if maxAllowed > 0 {
		return count < maxAllowed, nil
	}
	return count == 0, nil
}

func upsertIPWhitelist(db *sql.DB, ip string, maxAccounts int) error {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return errors.New("INVALID_IP")
	}
	if maxAccounts <= 0 {
		maxAccounts = 2
	}
	_, err := db.Exec(`
		INSERT INTO ip_whitelist (ip, max_accounts, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (ip)
		DO UPDATE SET max_accounts = EXCLUDED.max_accounts
	`, ip, maxAccounts, time.Now().UTC())
	return err
}

func removeIPWhitelist(db *sql.DB, ip string) error {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return errors.New("INVALID_IP")
	}
	_, err := db.Exec(`
		DELETE FROM ip_whitelist
		WHERE ip = $1
	`, ip)
	return err
}

func createIPWhitelistRequest(db *sql.DB, ip string, accountID string, reason string) error {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return errors.New("INVALID_IP")
	}
	reason = strings.TrimSpace(reason)
	requestID, err := randomToken(16)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO ip_whitelist_requests (
			request_id,
			ip,
			account_id,
			reason,
			status,
			created_at
		)
		VALUES ($1, $2, $3, $4, 'pending', $5)
	`, requestID, ip, accountID, reason, time.Now().UTC())
	return err
}

func hasPendingWhitelistRequest(db *sql.DB, ip string) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM ip_whitelist_requests
			WHERE ip = $1 AND status = 'pending'
		)
	`, strings.TrimSpace(ip)).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

type WhitelistRequest struct {
	RequestID string
	IP        string
	AccountID string
	Reason    string
	CreatedAt time.Time
}

func listPendingWhitelistRequests(db *sql.DB) ([]WhitelistRequest, error) {
	rows, err := db.Query(`
		SELECT request_id, ip, COALESCE(account_id, ''), COALESCE(reason, ''), created_at
		FROM ip_whitelist_requests
		WHERE status = 'pending'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requests := []WhitelistRequest{}
	for rows.Next() {
		var req WhitelistRequest
		if err := rows.Scan(&req.RequestID, &req.IP, &req.AccountID, &req.Reason, &req.CreatedAt); err != nil {
			continue
		}
		requests = append(requests, req)
	}
	return requests, nil
}

func resolveWhitelistRequest(db *sql.DB, requestID string, status string, resolvedBy string) error {
	status = strings.TrimSpace(strings.ToLower(status))
	if status != "approved" && status != "denied" {
		return errors.New("INVALID_STATUS")
	}
	_, err := db.Exec(`
		UPDATE ip_whitelist_requests
		SET status = $2, resolved_at = $3, resolved_by = $4
		WHERE request_id = $1
	`, requestID, status, time.Now().UTC(), resolvedBy)
	return err
}
