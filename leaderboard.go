package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type leaderboardFilters struct {
	SeasonID    string
	Query       string
	IncludeBots bool
	BotOnly     bool
	Sort        string
	Page        int
	PageSize    int
}

func leaderboardHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		filters := parseLeaderboardFilters(r)
		orderBy := leaderboardOrderBy(filters.Sort)

		whereClauses := []string{"1=1"}
		args := []interface{}{filters.SeasonID}
		argIndex := 2

		if filters.BotOnly {
			whereClauses = append(whereClauses, "p.is_bot = true")
		} else if !filters.IncludeBots {
			whereClauses = append(whereClauses, "p.is_bot = false")
		}

		if filters.Query != "" {
			whereClauses = append(whereClauses, "(a.username ILIKE $"+strconv.Itoa(argIndex)+" OR a.display_name ILIKE $"+strconv.Itoa(argIndex)+")")
			args = append(args, "%"+filters.Query+"%")
			argIndex++
		}

		baseCTE := fmt.Sprintf(`
			WITH player_stats AS (
				SELECT
					p.player_id,
					p.coins,
					p.stars,
					p.created_at,
					p.is_bot,
					p.bot_profile,
					COALESCE(a.display_name, a.username, p.player_id) AS display_name,
					COALESCE(SUM(CASE WHEN ($1 = '' OR spl.season_id = $1) THEN spl.price_paid ELSE 0 END), 0) AS coins_spent_lifetime,
					MAX(CASE WHEN ($1 = '' OR spl.season_id = $1) THEN spl.created_at ELSE NULL END) AS last_star_acquired_at
				FROM players p
				LEFT JOIN accounts a ON a.player_id = p.player_id
				LEFT JOIN star_purchase_log spl ON spl.player_id = p.player_id
				WHERE %s
				GROUP BY p.player_id, p.coins, p.stars, p.created_at, p.is_bot, p.bot_profile, a.display_name, a.username
			)
		`, strings.Join(whereClauses, " AND "))

		countQuery := baseCTE + "SELECT COUNT(*) FROM player_stats"
		var total int
		if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}

		offset := (filters.Page - 1) * filters.PageSize
		argsWithPage := append(args, filters.PageSize, offset)
		resultsQuery := fmt.Sprintf(`
			%s
			SELECT
				ROW_NUMBER() OVER (ORDER BY %s) AS rank,
				player_id,
				display_name,
				stars,
				coins_spent_lifetime,
				last_star_acquired_at,
				is_bot,
				bot_profile
			FROM player_stats
			ORDER BY %s
			LIMIT $%d OFFSET $%d
		`, baseCTE, orderBy, orderBy, len(args)+1, len(args)+2)

		rows, err := db.Query(resultsQuery, argsWithPage...)
		if err != nil {
			json.NewEncoder(w).Encode(SimpleResponse{OK: false, Error: "INTERNAL_ERROR"})
			return
		}
		defer rows.Close()

		results := []LeaderboardEntry{}
		for rows.Next() {
			var entry LeaderboardEntry
			var lastStar sql.NullTime
			var botProfile sql.NullString
			if err := rows.Scan(&entry.Rank, &entry.PlayerID, &entry.DisplayName, &entry.Stars, &entry.CoinsSpentLifetime, &lastStar, &entry.IsBot, &botProfile); err != nil {
				continue
			}
			if lastStar.Valid {
				entry.LastStarAcquiredAt = lastStar.Time.UTC().Format(time.RFC3339)
			}
			if botProfile.Valid {
				entry.BotProfile = botProfile.String
			}
			results = append(results, entry)
		}

		json.NewEncoder(w).Encode(LeaderboardResponse{
			Page:     filters.Page,
			PageSize: filters.PageSize,
			Total:    total,
			Results:  results,
		})
	}
}

func parseLeaderboardFilters(r *http.Request) leaderboardFilters {
	query := r.URL.Query()
	page := parsePositiveInt(query.Get("page"), 1)
	pageSize := parsePositiveInt(query.Get("pageSize"), 50)
	if pageSize > 200 {
		pageSize = 200
	}
	includeBots := true
	if raw := strings.TrimSpace(query.Get("includeBots")); raw != "" {
		if parsed, err := parseBool(raw); err == nil {
			includeBots = parsed
		}
	}
	botOnly := false
	if raw := strings.TrimSpace(query.Get("botOnly")); raw != "" {
		if parsed, err := parseBool(raw); err == nil {
			botOnly = parsed
		}
	}

	return leaderboardFilters{
		SeasonID:    strings.TrimSpace(query.Get("seasonId")),
		Query:       strings.TrimSpace(query.Get("q")),
		IncludeBots: includeBots,
		BotOnly:     botOnly,
		Sort:        strings.TrimSpace(query.Get("sort")),
		Page:        page,
		PageSize:    pageSize,
	}
}

func leaderboardOrderBy(sortKey string) string {
	switch sortKey {
	case "stars_asc":
		return "stars ASC, last_star_acquired_at ASC NULLS LAST, coins_spent_lifetime ASC, created_at ASC, player_id ASC"
	case "coins_spent_asc":
		return "coins_spent_lifetime ASC, stars DESC, last_star_acquired_at ASC NULLS LAST, created_at ASC, player_id ASC"
	case "coins_spent_desc":
		return "coins_spent_lifetime DESC, stars DESC, last_star_acquired_at ASC NULLS LAST, created_at ASC, player_id ASC"
	case "last_star_time_asc":
		return "last_star_acquired_at ASC NULLS LAST, stars DESC, coins_spent_lifetime ASC, created_at ASC, player_id ASC"
	case "created_at_asc":
		return "created_at ASC, player_id ASC"
	case "stars_desc", "":
		fallthrough
	default:
		return "stars DESC, last_star_acquired_at ASC NULLS LAST, coins_spent_lifetime ASC, created_at ASC, player_id ASC"
	}
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
