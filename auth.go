package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	sessionTTL      = 7 * 24 * time.Hour
	accessTokenTTL  = 30 * time.Minute
	refreshTokenTTL = 60 * 24 * time.Hour
)

type Account struct {
	AccountID    string
	Username     string
	DisplayName  string
	PlayerID     string
	Email        string
	Bio          string
	Pronouns     string
	Location     string
	Website      string
	AvatarURL    string
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
	var bio sql.NullString
	var pronouns sql.NullString
	var location sql.NullString
	var website sql.NullString
	var avatarURL sql.NullString
	if err := db.QueryRow(`
		SELECT account_id, username, display_name, player_id, password_hash, admin_key_hash, role, email,
			bio, pronouns, location, website, avatar_url
		FROM accounts
		WHERE username = $1
	`, username).Scan(&account.AccountID, &account.Username, &account.DisplayName, &account.PlayerID, &hash, &adminKey, &role, &email, &bio, &pronouns, &location, &website, &avatarURL); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("INVALID_CREDENTIALS")
		}
		return nil, err
	}
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
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		token := strings.TrimSpace(auth[len("bearer "):])
		if token != "" {
			accountID, err := verifyAccessToken(token)
			if err != nil {
				return nil, "", err
			}
			account, err := loadAccountByID(db, accountID)
			return account, "", err
		}
	}

	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil, "", err
	}

	var account Account
	var expiresAt time.Time
	var adminKey sql.NullString
	var role string
	var email sql.NullString
	var bio sql.NullString
	var pronouns sql.NullString
	var location sql.NullString
	var website sql.NullString
	var avatarURL sql.NullString
	if err := db.QueryRow(`
		SELECT a.account_id, a.username, a.display_name, a.player_id, a.admin_key_hash, a.role, a.email,
			a.bio, a.pronouns, a.location, a.website, a.avatar_url, s.expires_at
		FROM sessions s
		JOIN accounts a ON a.account_id = s.account_id
		WHERE s.session_id = $1
	`, cookie.Value).Scan(&account.AccountID, &account.Username, &account.DisplayName, &account.PlayerID, &adminKey, &role, &email, &bio, &pronouns, &location, &website, &avatarURL, &expiresAt); err != nil {
		return nil, "", err
	}
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
	account.Role = normalizeRole(role)

	if time.Now().UTC().After(expiresAt) {
		clearSession(db, cookie.Value)
		return nil, "", errors.New("SESSION_EXPIRED")
	}

	return &account, cookie.Value, nil
}

func loadAccountByID(db *sql.DB, accountID string) (*Account, error) {
	var account Account
	var adminKey sql.NullString
	var role string
	var email sql.NullString
	var bio sql.NullString
	var pronouns sql.NullString
	var location sql.NullString
	var website sql.NullString
	var avatarURL sql.NullString
	if err := db.QueryRow(`
		SELECT account_id, username, display_name, player_id, admin_key_hash, role, email,
			bio, pronouns, location, website, avatar_url
		FROM accounts
		WHERE account_id = $1
	`, accountID).Scan(&account.AccountID, &account.Username, &account.DisplayName, &account.PlayerID, &adminKey, &role, &email, &bio, &pronouns, &location, &website, &avatarURL); err != nil {
		return nil, err
	}
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
	account.Role = normalizeRole(role)
	return &account, nil
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

func accessTokenSecret() []byte {
	secret := strings.TrimSpace(os.Getenv("ACCESS_TOKEN_SECRET"))
	if secret == "" {
		secret = "dev-insecure-access-token-secret"
	}
	return []byte(secret)
}

func issueAccessToken(accountID string, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = accessTokenTTL
	}
	expiresAt := time.Now().UTC().Add(ttl)
	nonce, err := randomToken(6)
	if err != nil {
		return "", time.Time{}, err
	}
	payload := accountID + "|" + strconv.FormatInt(expiresAt.Unix(), 10) + "|" + nonce
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))

	mac := hmac.New(sha256.New, accessTokenSecret())
	mac.Write([]byte(encoded))
	sig := hex.EncodeToString(mac.Sum(nil))

	return encoded + "." + sig, expiresAt, nil
}

func verifyAccessToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", errors.New("INVALID_TOKEN")
	}
	encoded := parts[0]
	sig := parts[1]

	mac := hmac.New(sha256.New, accessTokenSecret())
	mac.Write([]byte(encoded))
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) != 1 {
		return "", errors.New("INVALID_TOKEN")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("INVALID_TOKEN")
	}
	partsPayload := strings.Split(string(payloadBytes), "|")
	if len(partsPayload) < 2 {
		return "", errors.New("INVALID_TOKEN")
	}
	accountID := partsPayload[0]
	exp, err := strconv.ParseInt(partsPayload[1], 10, 64)
	if err != nil {
		return "", errors.New("INVALID_TOKEN")
	}
	if time.Now().UTC().After(time.Unix(exp, 0)) {
		return "", errors.New("TOKEN_EXPIRED")
	}
	return accountID, nil
}

type sqlExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func createRefreshToken(db sqlExecer, accountID string, purpose string, userAgent string, ip string) (string, time.Time, error) {
	if purpose == "" {
		purpose = "auth"
	}
	raw, err := randomToken(32)
	if err != nil {
		return "", time.Time{}, err
	}
	hash := hashToken(raw)
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(refreshTokenTTL)
	_, err = db.Exec(`
		INSERT INTO refresh_tokens (
			account_id,
			token_hash,
			issued_at,
			expires_at,
			user_agent,
			ip,
			purpose
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, accountID, hash, issuedAt, expiresAt, userAgent, ip, purpose)
	if err != nil {
		return "", time.Time{}, err
	}
	return raw, expiresAt, nil
}

type refreshTokenRecord struct {
	AccountID string
	ExpiresAt time.Time
	RevokedAt sql.NullTime
}

func rotateRefreshToken(db *sql.DB, rawToken string, userAgent string, ip string) (string, time.Time, string, time.Time, error) {
	hash := hashToken(rawToken)
	var record refreshTokenRecord

	tx, err := db.Begin()
	if err != nil {
		return "", time.Time{}, "", time.Time{}, err
	}
	defer tx.Rollback()

	if err := tx.QueryRow(`
		SELECT account_id, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, hash).Scan(&record.AccountID, &record.ExpiresAt, &record.RevokedAt); err != nil {
		if err == sql.ErrNoRows {
			return "", time.Time{}, "", time.Time{}, errors.New("INVALID_REFRESH_TOKEN")
		}
		return "", time.Time{}, "", time.Time{}, err
	}
	if record.RevokedAt.Valid {
		return "", time.Time{}, "", time.Time{}, errors.New("REFRESH_TOKEN_REVOKED")
	}
	if time.Now().UTC().After(record.ExpiresAt) {
		return "", time.Time{}, "", time.Time{}, errors.New("REFRESH_TOKEN_EXPIRED")
	}

	_, err = tx.Exec(`
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1
	`, hash)
	if err != nil {
		return "", time.Time{}, "", time.Time{}, err
	}

	newRefresh, newRefreshExpires, err := createRefreshToken(tx, record.AccountID, "auth", userAgent, ip)
	if err != nil {
		return "", time.Time{}, "", time.Time{}, err
	}

	accessToken, accessExpires, err := issueAccessToken(record.AccountID, accessTokenTTL)
	if err != nil {
		return "", time.Time{}, "", time.Time{}, err
	}

	if err := tx.Commit(); err != nil {
		return "", time.Time{}, "", time.Time{}, err
	}

	return accessToken, accessExpires, newRefresh, newRefreshExpires, nil
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

func generateAdminKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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
