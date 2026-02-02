package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
)

const sessionTTL = 7 * 24 * time.Hour

type Account struct {
	AccountID   string
	Username    string
	DisplayName string
	PlayerID    string
}

func createAccount(db *sql.DB, username string, password string, displayName string) (*Account, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if len(username) < 3 || len(username) > 32 {
		return nil, errors.New("INVALID_USERNAME")
	}
	if len(password) < 8 || len(password) > 128 {
		return nil, errors.New("INVALID_PASSWORD")
	}
	if displayName == "" {
		displayName = username
	}

	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	accountID, err := randomToken(16)
	if err != nil {
		return nil, err
	}
	playerID, err := randomToken(16)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		INSERT INTO accounts (
			account_id,
			username,
			password_hash,
			display_name,
			player_id,
			created_at,
			last_login_at
		)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	`, accountID, username, hash, displayName, playerID)
	if err != nil {
		return nil, err
	}

	return &Account{
		AccountID:   accountID,
		Username:    username,
		DisplayName: displayName,
		PlayerID:    playerID,
	}, nil
}

func authenticate(db *sql.DB, username string, password string) (*Account, error) {
	username = strings.TrimSpace(strings.ToLower(username))

	var account Account
	var hash string
	if err := db.QueryRow(`
		SELECT account_id, username, display_name, player_id, password_hash
		FROM accounts
		WHERE username = $1
	`, username).Scan(&account.AccountID, &account.Username, &account.DisplayName, &account.PlayerID, &hash); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("INVALID_CREDENTIALS")
		}
		return nil, err
	}

	if !verifyPassword(hash, password) {
		return nil, errors.New("INVALID_CREDENTIALS")
	}

	_, _ = db.Exec(`
		UPDATE accounts
		SET last_login_at = NOW()
		WHERE account_id = $1
	`, account.AccountID)

	return &account, nil
}

func createSession(db *sql.DB, accountID string) (string, time.Time, error) {
	sessionID, err := randomToken(24)
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := time.Now().UTC().Add(sessionTTL)
	_, err = db.Exec(`
		INSERT INTO sessions (session_id, account_id, expires_at)
		VALUES ($1, $2, $3)
	`, sessionID, accountID, expiresAt)
	if err != nil {
		return "", time.Time{}, err
	}

	return sessionID, expiresAt, nil
}

func clearSession(db *sql.DB, sessionID string) {
	_, _ = db.Exec(`
		DELETE FROM sessions
		WHERE session_id = $1
	`, sessionID)
}

func getSessionAccount(db *sql.DB, r *http.Request) (*Account, string, error) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil, "", err
	}

	var account Account
	var expiresAt time.Time
	if err := db.QueryRow(`
		SELECT a.account_id, a.username, a.display_name, a.player_id, s.expires_at
		FROM sessions s
		JOIN accounts a ON a.account_id = s.account_id
		WHERE s.session_id = $1
	`, cookie.Value).Scan(&account.AccountID, &account.Username, &account.DisplayName, &account.PlayerID, &expiresAt); err != nil {
		return nil, "", err
	}

	if time.Now().UTC().After(expiresAt) {
		clearSession(db, cookie.Value)
		return nil, "", errors.New("SESSION_EXPIRED")
	}

	return &account, cookie.Value, nil
}

func writeSessionCookie(w http.ResponseWriter, sessionID string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func randomToken(bytesLen int) (string, error) {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashPassword(password string) (string, error) {
	salt, err := randomToken(16)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(salt + password))
	hash := base64.RawURLEncoding.EncodeToString(sum[:])
	return salt + ":" + hash, nil
}

func verifyPassword(stored string, password string) bool {
	parts := strings.Split(stored, ":")
	if len(parts) != 2 {
		return false
	}
	salt := parts[0]
	encoded := parts[1]

	sum := sha256.Sum256([]byte(salt + password))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(encoded)) == 1
}
