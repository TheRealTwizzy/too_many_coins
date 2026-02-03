package main

import (
	"database/sql"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GlobalSettings struct {
	ActiveDripIntervalSeconds int
	IdleDripIntervalSeconds   int
	ActiveDripAmount          int
	IdleDripAmount            int
	ActivityWindowSeconds     int
	DripEnabled               bool
	BotsEnabled               bool
	BotMinStarIntervalSeconds int
}

var (
	settingsMu     sync.RWMutex
	cachedSettings = GlobalSettings{
		ActiveDripIntervalSeconds: 60,
		IdleDripIntervalSeconds:   240,
		ActiveDripAmount:          2,
		IdleDripAmount:            1,
		ActivityWindowSeconds:     120,
		DripEnabled:               true,
		BotsEnabled:               true,
		BotMinStarIntervalSeconds: 90,
	}
)

func LoadGlobalSettings(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT key, value
		FROM global_settings
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	settingsMu.Lock()
	defer settingsMu.Unlock()

	for rows.Next() {
		var key string
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		applySetting(&cachedSettings, key, value)
	}
	return nil
}

func GetGlobalSettings() GlobalSettings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return cachedSettings
}

func UpdateGlobalSettings(db *sql.DB, updates map[string]string) (GlobalSettings, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	for key, value := range updates {
		applySetting(&cachedSettings, key, value)
		_, err := db.Exec(`
			INSERT INTO global_settings (key, value, updated_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
		`, key, value)
		if err != nil {
			return cachedSettings, err
		}
	}
	return cachedSettings, nil
}

func applySetting(target *GlobalSettings, key string, value string) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "active_drip_interval_seconds":
		if v, err := strconv.Atoi(value); err == nil {
			target.ActiveDripIntervalSeconds = v
		}
	case "idle_drip_interval_seconds":
		if v, err := strconv.Atoi(value); err == nil {
			target.IdleDripIntervalSeconds = v
		}
	case "active_drip_amount":
		if v, err := strconv.Atoi(value); err == nil {
			target.ActiveDripAmount = v
		}
	case "idle_drip_amount":
		if v, err := strconv.Atoi(value); err == nil {
			target.IdleDripAmount = v
		}
	case "activity_window_seconds":
		if v, err := strconv.Atoi(value); err == nil {
			target.ActivityWindowSeconds = v
		}
	case "drip_enabled":
		if v, err := parseBool(value); err == nil {
			target.DripEnabled = v
		}
	case "bots_enabled":
		if v, err := parseBool(value); err == nil {
			target.BotsEnabled = v
		}
	case "bot_min_star_interval_seconds":
		if v, err := strconv.Atoi(value); err == nil {
			target.BotMinStarIntervalSeconds = v
		}
	}
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	default:
		return false, strconv.ErrSyntax
	}
}

func ActiveActivityWindow() time.Duration {
	settings := GetGlobalSettings()
	if settings.ActivityWindowSeconds <= 0 {
		return 2 * time.Minute
	}
	return time.Duration(settings.ActivityWindowSeconds) * time.Second
}
