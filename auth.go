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
	AccountID    string
	Username     string
	DisplayName  string
	PlayerID     string
	Email        string
	AdminKeyHash string
	Role         string
}

func createAccount(db *sql.DB, username string, password string, displayName string, email string) (*Account, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if len(username) < 3 || len(username) > 32 {
		return nil, errors.New("INVALID_USERNAME")
	}
	if len(password) < 8 || len(password) > 128 {
		return nil, errors.New("INVALID_PASSWORD")
	}
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return nil, err
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
			email,
			role,
			created_at,
			last_login_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'user', NOW(), NOW())
	`, accountID, username, hash, displayName, playerID, normalizedEmail)
	if err != nil {
		return nil, err
	}

	return &Account{
		AccountID:   accountID,
		Username:    username,
		DisplayName: displayName,
		PlayerID:    playerID,
		Email:       normalizedEmail,
	}, nil
}

func authenticate(db *sql.DB, username string, password string) (*Account, error) {
	username = strings.TrimSpace(strings.ToLower(username))

	var account Account
	var hash string
	var adminKey sql.NullString
	var role string
	var email sql.NullString
	if err := db.QueryRow(`
		SELECT account_id, username, display_name, player_id, password_hash, admin_key_hash, role, email
		FROM accounts
		WHERE username = $1
	`, username).Scan(&account.AccountID, &account.Username, &account.DisplayName, &account.PlayerID, &hash, &adminKey, &role, &email); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("INVALID_CREDENTIALS")
		}
		return nil, err
	}
	if email.Valid {
		account.Email = email.String
	}
	if adminKey.Valid {
		account.AdminKeyHash = adminKey.String
	}
	account.Role = normalizeRole(role)

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
	var adminKey sql.NullString
	var role string
	var email sql.NullString
	if err := db.QueryRow(`
		SELECT a.account_id, a.username, a.display_name, a.player_id, a.admin_key_hash, a.role, a.email, s.expires_at
		FROM sessions s
		JOIN accounts a ON a.account_id = s.account_id
		WHERE s.session_id = $1
	`, cookie.Value).Scan(&account.AccountID, &account.Username, &account.DisplayName, &account.PlayerID, &adminKey, &role, &email, &expiresAt); err != nil {
		return nil, "", err
	}
	if email.Valid {
		account.Email = email.String
	}
	if adminKey.Valid {
		account.AdminKeyHash = adminKey.String
	}
	account.Role = normalizeRole(role)

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

func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", nil
	}
	if len(email) < 5 || len(email) > 254 {
		return "", errors.New("INVALID_EMAIL")
	}
	if !strings.Contains(email, "@") {
		return "", errors.New("INVALID_EMAIL")
	}
	return email, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func createPasswordResetToken(db *sql.DB, accountID string) (string, error) {
	resetID, err := randomToken(16)
	if err != nil {
		return "", err
	}
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().UTC().Add(1 * time.Hour)
	_, err = db.Exec(`
		INSERT INTO password_resets (
			reset_id,
			account_id,
			token_hash,
			expires_at,
			created_at
		)
		VALUES ($1, $2, $3, $4, NOW())
	`, resetID, accountID, hashToken(token), expiresAt)
	if err != nil {
		return "", err
	}
	return token, nil
}

func resetPasswordWithToken(db *sql.DB, token string, newPassword string) error {
	if len(newPassword) < 8 || len(newPassword) > 128 {
		return errors.New("INVALID_PASSWORD")
	}
	if token == "" {
		return errors.New("INVALID_TOKEN")
	}
	hash := hashToken(token)
	var accountID string
	var expiresAt time.Time
	var usedAt sql.NullTime
	err := db.QueryRow(`
		SELECT account_id, expires_at, used_at
		FROM password_resets
		WHERE token_hash = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, hash).Scan(&accountID, &expiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return errors.New("INVALID_TOKEN")
	}
	if err != nil {
		return err
	}
	if usedAt.Valid {
		return errors.New("TOKEN_USED")
	}
	if time.Now().UTC().After(expiresAt) {
		return errors.New("TOKEN_EXPIRED")
	}
	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		UPDATE accounts
		SET password_hash = $2
		WHERE account_id = $1
	`, accountID, passwordHash)
	if err != nil {
		return err
	}
	_, _ = db.Exec(`
		UPDATE password_resets
		SET used_at = NOW()
		WHERE token_hash = $1
	`, hash)
	return nil
}

func lookupAccountForReset(db *sql.DB, identifier string) (*Account, error) {
	identifier = strings.TrimSpace(strings.ToLower(identifier))
	if identifier == "" {
		return nil, errors.New("INVALID_REQUEST")
	}
	var account Account
	var email sql.NullString
	row := db.QueryRow(`
		SELECT account_id, username, display_name, player_id, email
		FROM accounts
		WHERE username = $1 OR email = $1
		LIMIT 1
	`, identifier)
	if err := row.Scan(&account.AccountID, &account.Username, &account.DisplayName, &account.PlayerID, &email); err != nil {
		return nil, err
	}
	if email.Valid {
		account.Email = email.String
	}
	return &account, nil
}

func setAdminKey(db *sql.DB, accountID string, key string) error {
	hash, err := hashPassword(key)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		UPDATE accounts
		SET admin_key_hash = $2
		WHERE account_id = $1
	`, accountID, hash)
	return err
}

func verifyAdminKey(stored string, provided string) bool {
	return verifyPassword(stored, provided)
}

func normalizeRole(role string) string {
	switch strings.ToLower(role) {
	case "admin", "moderator":
		return strings.ToLower(role)
	default:
		return "user"
	}
}

func setAccountRole(db *sql.DB, accountID string, role string) error {
	role = normalizeRole(role)
	_, err := db.Exec(`
		UPDATE accounts
		SET role = $2
		WHERE account_id = $1
	`, accountID, role)
	return err
}

func setAccountRoleByUsername(db *sql.DB, username string, role string) error {
	role = normalizeRole(role)
	_, err := db.Exec(`
		UPDATE accounts
		SET role = $2
		WHERE username = $1
	`, strings.ToLower(username), role)
	return err
}

func setAdminKeyByUsername(db *sql.DB, username string, key string) error {
	hash, err := hashPassword(key)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		UPDATE accounts
		SET admin_key_hash = $2
		WHERE username = $1
	`, strings.ToLower(username), hash)
	return err
}
