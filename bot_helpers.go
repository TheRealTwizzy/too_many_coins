package main

import (
	"database/sql"
	"os"
	"strings"
	"time"
)

func botsEnabled() bool {
	settings := GetGlobalSettings()
	if !settings.BotsEnabled {
		return false
	}
	value := strings.TrimSpace(strings.ToLower(os.Getenv("BOTS_ENABLED")))
	if value == "" {
		return true
	}
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func botMinStarInterval() time.Duration {
	settings := GetGlobalSettings()
	if settings.BotMinStarIntervalSeconds <= 0 {
		return 90 * time.Second
	}
	return time.Duration(settings.BotMinStarIntervalSeconds) * time.Second
}

func getPlayerBotInfo(db *sql.DB, playerID string) (bool, string, error) {
	var isBot bool
	var profile sql.NullString
	if err := db.QueryRow(`
		SELECT is_bot, bot_profile
		FROM players
		WHERE player_id = $1
	`, playerID).Scan(&isBot, &profile); err != nil {
		return false, "", err
	}
	return isBot, profile.String, nil
}

func enforceBotStarRateLimit(db *sql.DB, playerID string, minInterval time.Duration) (bool, int, error) {
	var last time.Time
	if err := db.QueryRow(`
		SELECT created_at
		FROM star_purchase_log
		WHERE player_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, playerID).Scan(&last); err != nil {
		if err == sql.ErrNoRows {
			return true, 0, nil
		}
		return false, 0, err
	}

	if time.Since(last) < minInterval {
		retry := int(minInterval.Seconds() - time.Since(last).Seconds())
		if retry < 1 {
			retry = 1
		}
		return false, retry, nil
	}
	return true, 0, nil
}
