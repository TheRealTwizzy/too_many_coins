package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"time"
)

const startupAdvisoryLockID int64 = 824173921

var startupLockConn *sql.Conn

func acquireStartupLock(ctx context.Context, db *sql.DB) (*sql.Conn, bool, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	var acquired bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, startupAdvisoryLockID).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, false, err
	}
	if !acquired {
		_ = conn.Close()
		return nil, false, nil
	}
	return conn, true, nil
}

func ensureAlphaAdmin(ctx context.Context, db *sql.DB) error {
	log.Printf("ensureAlphaAdmin entered (phase=%s)", CurrentPhase())
	if CurrentPhase() != PhaseAlpha {
		return nil
	}

	const username = "alpha-admin"
	const displayName = "Alpha Admin"
	const email = ""

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	bootstrapComplete := false
	var bootstrapValue string
	if err := tx.QueryRowContext(ctx, `
		SELECT value
		FROM global_settings
		WHERE key = 'admin_bootstrap_complete'
		FOR UPDATE
	`).Scan(&bootstrapValue); err == nil {
		bootstrapComplete = strings.ToLower(strings.TrimSpace(bootstrapValue)) == "true"
	} else if err != sql.ErrNoRows {
		return err
	}

	var adminAccountID string
	adminErr := tx.QueryRowContext(ctx, `
		SELECT account_id
		FROM accounts
		WHERE role IN ('admin', 'frozen:admin')
		LIMIT 1
		FOR UPDATE
	`).Scan(&adminAccountID)
	if adminErr == nil {
		if !bootstrapComplete {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO global_settings (key, value, updated_at)
				VALUES ('admin_bootstrap_complete', 'true', NOW())
				ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
			`); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		log.Println("Alpha admin bootstrap: admin already exists, skipping")
		return nil
	}
	if adminErr != sql.ErrNoRows {
		return adminErr
	}
	if bootstrapComplete {
		return errors.New("bootstrap sealed but no admin exists; refuse to start")
	}

	var existingAccountID string
	if err := tx.QueryRowContext(ctx, `
		SELECT account_id
		FROM accounts
		WHERE username = $1
		LIMIT 1
		FOR UPDATE
	`, username).Scan(&existingAccountID); err == nil {
		return errors.New("bootstrap admin username already exists without admin role")
	} else if err != sql.ErrNoRows {
		return err
	}

	bootstrapPassword := strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_PASSWORD"))
	if bootstrapPassword == "" {
		return errors.New("ADMIN_BOOTSTRAP_PASSWORD required for alpha bootstrap")
	}
	if len(bootstrapPassword) < 8 || len(bootstrapPassword) > 128 {
		return errors.New("ADMIN_BOOTSTRAP_PASSWORD must be 8-128 characters")
	}

	accountID, err := randomToken(16)
	if err != nil {
		return err
	}
	playerID, err := randomToken(16)
	if err != nil {
		return err
	}
	passwordHash, err := hashPassword(bootstrapPassword)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO players (player_id, coins, stars, created_at, last_active_at)
		VALUES ($1, 0, 0, NOW(), NOW())
	`, playerID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO accounts (
			account_id,
			username,
			password_hash,
			display_name,
			player_id,
			email,
			role,
			must_change_password,
			created_at,
			last_login_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'admin', TRUE, NOW(), NOW())
	`, accountID, username, passwordHash, displayName, playerID, email); err != nil {
		return err
	}

	bootstrapDetails := map[string]interface{}{
		"username":    username,
		"displayName": displayName,
	}
	payload, err := json.Marshal(bootstrapDetails)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO admin_audit_log (admin_account_id, action_type, scope_type, scope_id, reason, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, accountID, "auto_admin_bootstrap", "account", accountID, "bootstrap", string(payload)); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO global_settings (key, value, updated_at)
		VALUES ('admin_bootstrap_complete', 'true', NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Println("Alpha admin bootstrap: created alpha-admin")
	return nil
}

func ensureActiveSeason(ctx context.Context, db *sql.DB) error {
	seasonID := currentSeasonID()
	_, err := db.ExecContext(ctx, `
		INSERT INTO season_economy (
			season_id,
			global_coin_pool,
			global_stars_purchased,
			coins_distributed,
			emission_remainder,
			market_pressure,
			price_floor,
			last_updated
		)
		VALUES ($1, 0, 0, 0, 0, 1.0, 0, NOW())
		ON CONFLICT (season_id) DO NOTHING
	`, seasonID)
	return err
}

func updateTickHeartbeat(db *sql.DB, now time.Time) {
	_, err := db.Exec(`
		INSERT INTO global_settings (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, "tick_last_utc", now.UTC().Format(time.RFC3339))
	if err != nil {
		log.Println("tick heartbeat update failed:", err)
	}
}

func claimTick(db *sql.DB, now time.Time) bool {
	value := now.UTC().Format(time.RFC3339)
	result, err := db.Exec(`
		INSERT INTO global_settings (key, value, updated_at)
		VALUES ('tick_last_utc', $1, NOW())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value, updated_at = NOW()
		WHERE global_settings.value < EXCLUDED.value
	`, value)
	if err != nil {
		log.Println("tick claim failed:", err)
		return false
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false
	}
	return rows > 0
}

func readTickHeartbeat(ctx context.Context, db *sql.DB) (time.Time, error) {
	var value string
	if err := db.QueryRowContext(ctx, `
		SELECT value
		FROM global_settings
		WHERE key = 'tick_last_utc'
	`).Scan(&value); err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, value)
}
