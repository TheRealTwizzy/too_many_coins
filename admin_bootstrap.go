package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const ownerClaimWindowSeconds int64 = 300
const ownerClaimAppSalt = "tmc-admin-claim-v1"

const ownerClaimCodeLength = 8

const adminBootstrapSettingKey = "admin_bootstrap_complete"

var errBootstrapComplete = errors.New("BOOTSTRAP_COMPLETE")
var errAdminAlreadyUnlocked = errors.New("ADMIN_ALREADY_UNLOCKED")

type AdminBootstrapStatusResponse struct {
	AdminLocked            bool  `json:"adminLocked"`
	WindowSecondsRemaining int64 `json:"windowSecondsRemaining,omitempty"`
}

type AdminBootstrapClaimRequest struct {
	Code        string `json:"code"`
	NewPassword string `json:"newPassword"`
}

type adminBootstrapAccount struct {
	AccountID          string
	Username           string
	MustChangePassword bool
}

func adminBootstrapStatusHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		bootstrapComplete, err := adminBootstrapComplete(db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AdminBootstrapStatusResponse{AdminLocked: false})
			return
		}

		account, count, err := loadBootstrapAdminAccount(db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AdminBootstrapStatusResponse{AdminLocked: false})
			return
		}
		if count != 1 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AdminBootstrapStatusResponse{AdminLocked: false})
			return
		}
		adminLocked := !bootstrapComplete && count == 1 && account.MustChangePassword
		response := AdminBootstrapStatusResponse{AdminLocked: adminLocked}
		if adminLocked {
			response.WindowSecondsRemaining = ownerClaimWindowSecondsRemaining(time.Now().UTC())
		}
		json.NewEncoder(w).Encode(response)
	}
}

func adminBootstrapClaimHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req AdminBootstrapClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_REQUEST"})
			return
		}

		newPassword := strings.TrimSpace(req.NewPassword)
		if len(newPassword) < 8 || len(newPassword) > 128 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_PASSWORD"})
			return
		}

		bootstrapComplete, err := adminBootstrapComplete(db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if bootstrapComplete {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "BOOTSTRAP_COMPLETE"})
			return
		}

		account, count, err := loadBootstrapAdminAccount(db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		if count == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "ADMIN_MISSING"})
			return
		}
		if count > 1 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "ADMIN_COUNT_INVALID"})
			return
		}
		if !account.MustChangePassword {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "ADMIN_ALREADY_UNLOCKED"})
			return
		}

		secret := strings.TrimSpace(os.Getenv("OWNER_CLAIM_SECRET"))
		if secret == "" {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "CLAIM_SECRET_MISSING"})
			return
		}

		now := time.Now().UTC()
		if !verifyOwnerClaimCode(secret, req.Code, now) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INVALID_CLAIM_CODE"})
			return
		}

		if err := finalizeAdminBootstrap(r, db, newPassword); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, errBootstrapComplete) || errors.Is(err, errAdminAlreadyUnlocked) {
				status = http.StatusForbidden
			}
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: err.Error()})
			return
		}

		json.NewEncoder(w).Encode(SimpleResponse{OK: true})
	}
}

func finalizeAdminBootstrap(r *http.Request, db *sql.DB, newPassword string) error {
	ctx := r.Context()
	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 8 || len(newPassword) > 128 {
		return errors.New("INVALID_PASSWORD")
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	bootstrapComplete, err := adminBootstrapCompleteTx(tx)
	if err != nil {
		return err
	}
	if bootstrapComplete {
		return errBootstrapComplete
	}

	account, count, err := loadBootstrapAdminAccountTx(tx)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("ADMIN_MISSING")
	}
	if count > 1 {
		return errors.New("ADMIN_COUNT_INVALID")
	}
	if !account.MustChangePassword {
		return errAdminAlreadyUnlocked
	}

	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts
		SET password_hash = $2, must_change_password = FALSE
		WHERE account_id = $1
	`, account.AccountID, passwordHash); err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]string{
		"username": account.Username,
		"ip":       strings.TrimSpace(getClientIP(r)),
	})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO admin_audit_log (admin_account_id, action_type, scope_type, scope_id, reason, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, account.AccountID, "admin_bootstrap_claim", "account", account.AccountID, "owner_claim_code", string(payload)); err != nil {
		return err
	}

	if err := setAdminBootstrapCompleteTx(tx, true); err != nil {
		return err
	}

	return tx.Commit()
}

func loadBootstrapAdminAccount(db *sql.DB) (adminBootstrapAccount, int, error) {
	rows, err := db.Query(`
		SELECT account_id, username, must_change_password
		FROM accounts
		WHERE role IN ('admin', 'frozen:admin')
		ORDER BY created_at ASC
		LIMIT 2
	`)
	if err != nil {
		return adminBootstrapAccount{}, 0, err
	}
	defer rows.Close()

	count := 0
	var account adminBootstrapAccount
	for rows.Next() {
		count++
		if count == 1 {
			if err := rows.Scan(&account.AccountID, &account.Username, &account.MustChangePassword); err != nil {
				return adminBootstrapAccount{}, count, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return adminBootstrapAccount{}, count, err
	}
	return account, count, nil
}

func loadBootstrapAdminAccountTx(tx *sql.Tx) (adminBootstrapAccount, int, error) {
	rows, err := tx.Query(`
		SELECT account_id, username, must_change_password
		FROM accounts
		WHERE role IN ('admin', 'frozen:admin')
		ORDER BY created_at ASC
		LIMIT 2
		FOR UPDATE
	`)
	if err != nil {
		return adminBootstrapAccount{}, 0, err
	}
	defer rows.Close()

	count := 0
	var account adminBootstrapAccount
	for rows.Next() {
		count++
		if count == 1 {
			if err := rows.Scan(&account.AccountID, &account.Username, &account.MustChangePassword); err != nil {
				return adminBootstrapAccount{}, count, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return adminBootstrapAccount{}, count, err
	}
	return account, count, nil
}

func adminBootstrapComplete(db *sql.DB) (bool, error) {
	var value string
	if err := db.QueryRow(`
		SELECT value
		FROM global_settings
		WHERE key = $1
	`, adminBootstrapSettingKey).Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return strings.ToLower(strings.TrimSpace(value)) == "true", nil
}

func adminBootstrapCompleteTx(tx *sql.Tx) (bool, error) {
	var value string
	if err := tx.QueryRow(`
		SELECT value
		FROM global_settings
		WHERE key = $1
		FOR UPDATE
	`, adminBootstrapSettingKey).Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return strings.ToLower(strings.TrimSpace(value)) == "true", nil
}

func setAdminBootstrapCompleteTx(tx *sql.Tx, complete bool) error {
	value := "false"
	if complete {
		value = "true"
	}
	_, err := tx.Exec(`
		INSERT INTO global_settings (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, adminBootstrapSettingKey, value)
	return err
}

func verifyOwnerClaimCode(secret string, code string, now time.Time) bool {
	secret = strings.TrimSpace(secret)
	normalized := normalizeClaimCode(code)
	if secret == "" || normalized == "" {
		return false
	}
	windowIndex := now.UTC().Unix() / ownerClaimWindowSeconds
	for offset := int64(0); offset <= 1; offset++ {
		expected, err := ownerClaimCodeForWindow(secret, windowIndex-offset)
		if err != nil {
			return false
		}
		expected = normalizeClaimCode(expected)
		if expected == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(normalized), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}

func ownerClaimCodeForWindow(secret string, windowIndex int64) (string, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	message := []byte(ownerClaimAppSalt + ":" + strconv.FormatInt(windowIndex, 10))
	if _, err := mac.Write(message); err != nil {
		return "", err
	}
	sum := mac.Sum(nil)
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:5])
	encoded = strings.ToUpper(encoded)
	if len(encoded) < ownerClaimCodeLength {
		encoded = encoded + strings.Repeat("A", ownerClaimCodeLength-len(encoded))
	}
	encoded = encoded[:ownerClaimCodeLength]
	return encoded[:4] + "-" + encoded[4:], nil
}

func normalizeClaimCode(code string) string {
	code = strings.TrimSpace(strings.ToUpper(code))
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func ownerClaimWindowSecondsRemaining(now time.Time) int64 {
	elapsed := now.UTC().Unix() % ownerClaimWindowSeconds
	if elapsed == 0 {
		return 0
	}
	return ownerClaimWindowSeconds - elapsed
}
