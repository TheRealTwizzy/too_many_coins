package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultSeasonID         = "season-1"
	defaultSeasonStartLag   = -21 * 24 * time.Hour
	alphaSeasonLengthDays   = 14
	alphaSeasonMaxDays      = 21
	betaSeasonLengthDays    = 28
	releaseSeasonLengthDays = 28

	globalSettingActiveSeasonID       = "active_season_id"
	globalSettingActiveSeasonStartUTC = "active_season_start_utc"
)

var (
	seasonStateMu       sync.RWMutex
	activeSeasonID      string
	activeSeasonStart   time.Time
	seasonStateLoaded   bool
	fallbackSeasonOnce  sync.Once
	fallbackSeasonStart time.Time
	seasonStateDB       *sql.DB
	seasonAdvanceMu     sync.Mutex
)

type SeasonState struct {
	SeasonID string
	StartUTC time.Time
}

type seasonStateQueryer interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

func BindSeasonStateDB(db *sql.DB) {
	seasonStateDB = db
}

func LoadSeasonState(db *sql.DB) error {
	if db == nil {
		return nil
	}
	state, ok, err := fetchActiveSeasonState(db)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	setActiveSeasonState(state, true)
	return nil
}

func ActiveSeasonState() (SeasonState, bool) {
	seasonStateMu.RLock()
	defer seasonStateMu.RUnlock()
	if !seasonStateLoaded || strings.TrimSpace(activeSeasonID) == "" || activeSeasonStart.IsZero() {
		return SeasonState{}, false
	}
	return SeasonState{SeasonID: activeSeasonID, StartUTC: activeSeasonStart}, true
}

func currentSeasonID() string {
	if state, ok := ActiveSeasonState(); ok {
		return state.SeasonID
	}
	return defaultSeasonID
}

func DefaultSeasonID() string {
	return defaultSeasonID
}

func SeasonStart() time.Time {
	return seasonStart()
}

func seasonStart() time.Time {
	if state, ok := ActiveSeasonState(); ok {
		return state.StartUTC
	}
	fallbackSeasonOnce.Do(func() {
		start := os.Getenv("SEASON_START_UTC")
		if start != "" {
			if parsed, err := time.Parse(time.RFC3339, start); err == nil {
				fallbackSeasonStart = parsed.UTC()
				return
			}
		}
		fallbackSeasonStart = time.Now().UTC().Add(defaultSeasonStartLag)
	})
	return fallbackSeasonStart
}

func seasonEnd() time.Time {
	return seasonStart().Add(seasonLength())
}

func isSeasonEnded(now time.Time) bool {
	maybeAutoAdvanceAlphaSeason(now)
	return isSeasonEndedRaw(now)
}

func isSeasonEndedRaw(now time.Time) bool {
	return !now.Before(seasonEnd())
}

func seasonSecondsRemaining(now time.Time) int64 {
	maybeAutoAdvanceAlphaSeason(now)
	remaining := seasonEnd().Sub(now)
	if remaining < 0 {
		return 0
	}
	return int64(remaining.Seconds())
}

func seasonLength() time.Duration {
	switch CurrentPhase() {
	case PhaseBeta:
		return time.Duration(betaSeasonLengthDays) * 24 * time.Hour
	case PhaseRelease:
		return time.Duration(releaseSeasonLengthDays) * 24 * time.Hour
	default:
		return alphaSeasonLength()
	}
}

func alphaSeasonLength() time.Duration {
	// Alpha is single-season and tightly bounded: default 14 days.
	// Extension to 21 days is allowed only with explicit env + reason (telemetry gaps).
	lengthDays := alphaSeasonLengthDays
	if extensionDays, ok := alphaSeasonExtensionDays(); ok {
		if extensionDays > alphaSeasonMaxDays {
			extensionDays = alphaSeasonMaxDays
		}
		if extensionDays > lengthDays {
			lengthDays = extensionDays
		}
	}
	return time.Duration(lengthDays) * 24 * time.Hour
}

func alphaSeasonExtensionDays() (int, bool) {
	value := strings.TrimSpace(os.Getenv("ALPHA_SEASON_EXTENSION_DAYS"))
	if value == "" {
		return 0, false
	}
	if strings.TrimSpace(os.Getenv("ALPHA_SEASON_EXTENSION_REASON")) == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func maybeAutoAdvanceAlphaSeason(now time.Time) {
	if CurrentPhase() != PhaseAlpha {
		return
	}
	if seasonStateDB == nil {
		return
	}
	seasonAdvanceMu.Lock()
	defer seasonAdvanceMu.Unlock()

	state, ok, err := fetchActiveSeasonState(seasonStateDB)
	if err != nil {
		log.Println("season state load failed:", err)
		return
	}
	if !ok {
		fallbackID := strings.TrimSpace(currentSeasonID())
		fallbackStart := seasonStart().UTC()
		if fallbackID == "" || fallbackStart.IsZero() {
			return
		}
		endTime := fallbackStart.Add(seasonLength())
		if now.Before(endTime) {
			if err := persistActiveSeasonStateDirect(seasonStateDB, SeasonState{SeasonID: fallbackID, StartUTC: fallbackStart}); err != nil {
				log.Println("season state seed failed:", err)
				return
			}
			setActiveSeasonState(SeasonState{SeasonID: fallbackID, StartUTC: fallbackStart}, true)
			return
		}
		if _, err := FinalizeSeason(seasonStateDB, fallbackID); err != nil {
			log.Println("season finalization failed:", err)
			return
		}
		emitNotification(seasonStateDB, NotificationInput{
			RecipientRole: NotificationRolePlayer,
			Category:      NotificationCategorySystem,
			Type:          "season_ended",
			Priority:      NotificationPriorityHigh,
			Message:       "Season has ended. Final results are available.",
			Payload: map[string]interface{}{
				"seasonId": fallbackID,
			},
			DedupKey:    "season_end:" + fallbackID,
			DedupWindow: 6 * time.Hour,
		})
		emitNotification(seasonStateDB, NotificationInput{
			RecipientRole: NotificationRoleAdmin,
			Category:      NotificationCategorySystem,
			Type:          "season_ended",
			Priority:      NotificationPriorityHigh,
			Message:       "Season finalized: " + fallbackID,
			Payload: map[string]interface{}{
				"seasonId": fallbackID,
			},
			DedupKey:    "season_end_admin:" + fallbackID,
			DedupWindow: 6 * time.Hour,
		})
		if err := advanceAlphaSeason(seasonStateDB, now, true); err != nil {
			log.Println("alpha auto-advance failed:", err)
		}
		return
	}

	endTime := state.StartUTC.Add(seasonLength())
	if now.Before(endTime) {
		return
	}

	if err := advanceAlphaSeason(seasonStateDB, now, true); err != nil {
		log.Println("alpha auto-advance failed:", err)
	}
}

func AdvanceAlphaSeasonOverride(db *sql.DB, now time.Time) error {
	seasonAdvanceMu.Lock()
	defer seasonAdvanceMu.Unlock()
	return advanceAlphaSeason(db, now, true)
}

func fetchActiveSeasonState(q seasonStateQueryer) (SeasonState, bool, error) {
	return fetchActiveSeasonStateWithLock(q, false)
}

func fetchActiveSeasonStateForUpdate(tx *sql.Tx) (SeasonState, bool, error) {
	return fetchActiveSeasonStateWithLock(tx, true)
}

func fetchActiveSeasonStateWithLock(q seasonStateQueryer, lock bool) (SeasonState, bool, error) {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE"
	}
	var idValue string
	if err := q.QueryRow(`
		SELECT value
		FROM global_settings
		WHERE key = $1`+lockClause+`
	`, globalSettingActiveSeasonID).Scan(&idValue); err != nil {
		if err == sql.ErrNoRows {
			return SeasonState{}, false, nil
		}
		return SeasonState{}, false, err
	}
	var startValue string
	if err := q.QueryRow(`
		SELECT value
		FROM global_settings
		WHERE key = $1`+lockClause+`
	`, globalSettingActiveSeasonStartUTC).Scan(&startValue); err != nil {
		if err == sql.ErrNoRows {
			return SeasonState{}, false, nil
		}
		return SeasonState{}, false, err
	}
	idValue = strings.TrimSpace(idValue)
	if idValue == "" {
		return SeasonState{}, false, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(startValue))
	if err != nil {
		return SeasonState{}, false, err
	}
	return SeasonState{SeasonID: idValue, StartUTC: parsed.UTC()}, true, nil
}

func persistActiveSeasonState(tx *sql.Tx, state SeasonState) error {
	if state.SeasonID == "" || state.StartUTC.IsZero() {
		return nil
	}
	_, err := tx.Exec(`
		INSERT INTO global_settings (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, globalSettingActiveSeasonID, state.SeasonID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO global_settings (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, globalSettingActiveSeasonStartUTC, state.StartUTC.UTC().Format(time.RFC3339))
	return err
}

func persistActiveSeasonStateDirect(db *sql.DB, state SeasonState) error {
	if db == nil {
		return nil
	}
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := persistActiveSeasonState(tx, state); err != nil {
		return err
	}
	return tx.Commit()
}

func setActiveSeasonState(state SeasonState, loaded bool) {
	seasonStateMu.Lock()
	defer seasonStateMu.Unlock()
	activeSeasonID = state.SeasonID
	activeSeasonStart = state.StartUTC.UTC()
	seasonStateLoaded = loaded
}

func advanceAlphaSeason(db *sql.DB, now time.Time, allowIfNoActive bool) error {
	if CurrentPhase() != PhaseAlpha {
		return nil
	}
	if db == nil {
		return nil
	}

	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	state, ok, err := fetchActiveSeasonStateForUpdate(tx)
	if err != nil {
		return err
	}
	if !ok && !allowIfNoActive {
		return ErrNoActiveSeason
	}
	if ok {
		endTime := state.StartUTC.Add(seasonLength())
		if now.Before(endTime) {
			return ErrActiveSeason
		}
		if _, err := FinalizeSeason(db, state.SeasonID); err != nil {
			return err
		}
	}

	newSeasonID := buildAlphaSeasonID(now)
	newStart := now.UTC()

	if _, err := tx.Exec(`
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
	`, newSeasonID); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		UPDATE players
		SET coins = 0,
			stars = 0,
			daily_earn_total = 0,
			last_earn_reset_at = NOW()
	`); err != nil {
		return err
	}

	if err := persistActiveSeasonState(tx, SeasonState{SeasonID: newSeasonID, StartUTC: newStart}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	setActiveSeasonState(SeasonState{SeasonID: newSeasonID, StartUTC: newStart}, true)
	resetSeasonRuntimeState()
	if _, err := LoadOrCalibrateSeason(db, newSeasonID); err != nil {
		log.Println("season calibration failed:", err)
	}

	if ok {
		emitNotification(db, NotificationInput{
			RecipientRole: NotificationRolePlayer,
			Category:      NotificationCategorySystem,
			Type:          "season_ended",
			Priority:      NotificationPriorityHigh,
			Message:       "Season has ended. Final results are available.",
			Payload: map[string]interface{}{
				"seasonId": state.SeasonID,
			},
			DedupKey:    "season_end:" + state.SeasonID,
			DedupWindow: 6 * time.Hour,
		})
		emitNotification(db, NotificationInput{
			RecipientRole: NotificationRoleAdmin,
			Category:      NotificationCategorySystem,
			Type:          "season_ended",
			Priority:      NotificationPriorityHigh,
			Message:       "Season finalized: " + state.SeasonID,
			Payload: map[string]interface{}{
				"seasonId": state.SeasonID,
			},
			DedupKey:    "season_end_admin:" + state.SeasonID,
			DedupWindow: 6 * time.Hour,
		})
	}

	return nil
}

func resetSeasonRuntimeState() {
	economy.mu.Lock()
	economy.globalCoinPool = 0
	economy.globalStarsPurchased = 0
	economy.coinsDistributed = 0
	economy.emissionRemainder = 0
	economy.marketPressure = 1.0
	economy.priceFloor = 0
	economy.coinsInWallets = 0
	economy.activeCoinsInWallets = 0
	economy.activePlayers = 0
	economy.mu.Unlock()

	seasonControlsMu.Lock()
	cachedSeasonControls = make(map[string]interface{})
	lastSeasonControlsLoad = time.Time{}
	seasonControlsMu.Unlock()
}

func buildAlphaSeasonID(now time.Time) string {
	stamp := now.UTC().Format("20060102-150405")
	return "season-" + stamp
}

var (
	ErrActiveSeason   = errors.New("ACTIVE_SEASON_EXISTS")
	ErrNoActiveSeason = errors.New("NO_ACTIVE_SEASON")
)
