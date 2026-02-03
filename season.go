package main

import (
	"os"
	"sync"
	"time"
)

const (
	seasonLength          = 28 * 24 * time.Hour
	defaultSeasonID       = "season-1"
	defaultSeasonStartLag = -21 * 24 * time.Hour
)

var (
	seasonStartOnce sync.Once
	seasonStartTime time.Time
)

func currentSeasonID() string {
	return defaultSeasonID
}

func seasonStart() time.Time {
	seasonStartOnce.Do(func() {
		start := os.Getenv("SEASON_START_UTC")
		if start != "" {
			if parsed, err := time.Parse(time.RFC3339, start); err == nil {
				seasonStartTime = parsed.UTC()
				return
			}
		}

		seasonStartTime = time.Now().UTC().Add(defaultSeasonStartLag)
	})

	return seasonStartTime
}

func seasonEnd() time.Time {
	return seasonStart().Add(seasonLength)
}

func isSeasonEnded(now time.Time) bool {
	return !now.Before(seasonEnd())
}

func seasonSecondsRemaining(now time.Time) int64 {
	remaining := seasonEnd().Sub(now)
	if remaining < 0 {
		return 0
	}
	return int64(remaining.Seconds())
}
