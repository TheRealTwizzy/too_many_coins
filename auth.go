package main

import (
	"context"
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

const (
	sessionTTL = 7 * 24 * time.Hour
)

type Account struct {
	AccountID          string
	Username           string
	DisplayName        string
	PlayerID           string
	Email              string
	Bio                string
	Pronouns           string
	Location           string
	Website            string
	AvatarURL          string
	AdminKeyHash       string
	Role               string
	MustChangePassword bool
}

//
// =======================
// ACCOUNT CREATION
// =======================
//

func createAccount(db *sql.DB, username, password, displayName, email string) (*Account, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if len(username) < 3 || len(username) > 32 {
		return nil, errors.New("INVALID_USERNAME")
	}
	if len(password) < 8 || len(password) > 128 {
		return nil, errors.New("INVALID_PASSWORD")
	}

	email, err := normalizeEmail(email)
	if err != nil {
		return nil, err
	}
	if displayName == "" {
		displayName = username
	}

	passwordHash, err := hashPassword(password)
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

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO players (
			player_id,
			coins,
			stars,
			created_at,
			last_active_at,
			last_coin_grant_at
		)
		VALUES ($1, 0, 0, NOW(), NOW(), NOW())
	`, playerID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`
		INSERT INTO accounts (
			account_id,
			username,
			password_hash,
			display_name,
			player_id,
			email,
			role,
			created_at,
			last_login_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'user', NOW(), NOW())
	`, accountID, username, passwordHash, displayName, playerID, email)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &Account{
		AccountID:   accountID,
		Username:    username,
		DisplayName: displayName,
		PlayerID:    playerID,
		Email:       email,
		Role:        "user",
	}, nil
}

//
// =======================
// AUTHENTICATION
// =======================
//

func authenticate(db *sql.DB, username, password string) (*Account, error) {
	username = strings.ToLower(strings.TrimSpace(username))

	var account Account
	var passwordHash string
	var role string
	var mustChangePassword bool
	var email sql.NullString
	var bio sql.NullString
	var pronouns sql.NullString
	var location sql.NullString
	var website sql.NullString
	var avatarURL sql.NullString
	var adminKey sql.NullString

	err := db.QueryRow(`
		SELECT account_id, username, display_name, player_id,
		       password_hash, role, must_change_password,
		       email, bio, pronouns, location, website, avatar_url, admin_key_hash
		FROM accounts
		WHERE username = $1
	`, username).Scan(
		&account.AccountID,
		&account.Username,
		&account.DisplayName,
		&account.PlayerID,
		&passwordHash,
		&role,
		&mustChangePassword,
		&email,
		&bio,
		&pronouns,
		&location,
		&website,
		&avatarURL,
		&adminKey,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("INVALID_CREDENTIALS")
	}
	if err != nil {
		return nil, err
	}

	if isFrozenRole(role) {
		return nil, errors.New("ACCOUNT_FROZEN")
	}

	if !verifyPassword(passwordHash, password) {
		return nil, errors.New("INVALID_CREDENTIALS")
	}
	if normalizeRole(role) == "admin" && mustChangePassword {
		return nil, errors.New("ADMIN_BOOTSTRAP_REQUIRED")
	}

	_, _ = db.Exec(`UPDATE accounts SET last_login_at = NOW() WHERE account_id = $1`, account.AccountID)

	account.Role = normalizeRole(role)
	account.MustChangePassword = mustChangePassword
	if email.Valid {
		account.Email = email.String
	}
	if bio.Valid {
		account.Bio = bio.String
	}
	if pronouns.Valid {
		account.Pronouns = pronouns.String
	}
	if location.Valid {
		account.Location = location.String
	}
	if website.Valid {
		account.Website = website.String
	}
	if avatarURL.Valid {
		account.AvatarURL = avatarURL.String
	}
	if adminKey.Valid {
		account.AdminKeyHash = adminKey.String
	}

	return &account, nil
}

//
// =======================
// SESSION MANAGEMENT
// =======================
//

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
	_, _ = db.Exec(`DELETE FROM sessions WHERE session_id = $1`, sessionID)
}

func getSessionAccount(db *sql.DB, r *http.Request) (*Account, string, error) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil, "", errors.New("NOT_AUTHENTICATED")
	}

	var account Account
	var expiresAt time.Time
	var role string
	var mustChangePassword bool

	err = db.QueryRow(`
		SELECT a.account_id, a.username, a.display_name, a.player_id,
		       a.role, a.must_change_password, s.expires_at
		FROM sessions s
		JOIN accounts a ON a.account_id = s.account_id
		WHERE s.session_id = $1
	`, cookie.Value).Scan(
		&account.AccountID,
		&account.Username,
		&account.DisplayName,
		&account.PlayerID,
		&role,
		&mustChangePassword,
		&expiresAt,
	)
	if err != nil {
		return nil, "", errors.New("NOT_AUTHENTICATED")
	}

	if time.Now().UTC().After(expiresAt) {
		clearSession(db, cookie.Value)
		return nil, "", errors.New("SESSION_EXPIRED")
	}

	if isFrozenRole(role) {
		return nil, "", errors.New("ACCOUNT_FROZEN")
	}

	account.Role = normalizeRole(role)
	account.MustChangePassword = mustChangePassword

	return &account, cookie.Value, nil
}

//
// =======================
// COOKIE HELPERS
// =======================
//

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
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

//
// =======================
// UTILITIES
// =======================
//

func randomToken(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashPassword(password string) (string, error) {
	salt, err := randomToken(16)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(salt + password))
	return salt + ":" + base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func verifyPassword(stored, password string) bool {
	parts := strings.Split(stored, ":")
	if len(parts) != 2 {
		return false
	}
	salt := parts[0]
	expected := parts[1]
	sum := sha256.Sum256([]byte(salt + password))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(expected)) == 1
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", nil
	}
	if !strings.Contains(email, "@") {
		return "", errors.New("INVALID_EMAIL")
	}
	return email, nil
}

func normalizeRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	role = strings.TrimPrefix(role, "frozen:")
	if role == "admin" || role == "moderator" {
		return role
	}
	return "user"
}

func isFrozenRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	return role == "frozen" || strings.HasPrefix(role, "frozen:")
}

func AdminExists(ctx context.Context, db *sql.DB) bool {
	var exists bool
	_ = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM accounts WHERE role = 'admin' OR role = 'frozen:admin'
		)
	`).Scan(&exists)
	return exists
}
