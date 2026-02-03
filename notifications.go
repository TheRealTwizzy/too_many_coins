package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	NotificationRolePlayer    = "player"
	NotificationRoleModerator = "moderator"
	NotificationRoleAdmin     = "admin"
)

const (
	NotificationCategoryEconomy      = "economy"
	NotificationCategoryPlayerAction = "player_action"
	NotificationCategoryMarket       = "market"
	NotificationCategoryAbuse        = "abuse"
	NotificationCategorySystem       = "system"
	NotificationCategoryAdmin        = "admin"
)

const (
	NotificationPriorityNormal   = "normal"
	NotificationPriorityHigh     = "high"
	NotificationPriorityCritical = "critical"
)

var notificationCategoryList = []string{
	NotificationCategoryEconomy,
	NotificationCategoryPlayerAction,
	NotificationCategoryMarket,
	NotificationCategoryAbuse,
	NotificationCategorySystem,
	NotificationCategoryAdmin,
}

const notificationAccessSQL = `
	AND (n.recipient_account_id IS NULL OR n.recipient_account_id = $1 OR (n.account_id IS NOT NULL AND n.account_id = $1))
	AND (
		(n.recipient_role IS NOT NULL AND (
			($2 = 'admin' AND n.recipient_role IN ('player','moderator','admin'))
			OR ($2 = 'moderator' AND n.recipient_role = 'moderator')
			OR ($2 = 'player' AND n.recipient_role = 'player')
		))
		OR (n.recipient_role IS NULL AND (
			n.target_role = 'all'
			OR (n.target_role = 'admin' AND $2 = 'admin')
			OR (n.target_role = 'moderator' AND ($2 = 'admin' OR $2 = 'moderator'))
			OR (n.target_role = 'user' AND ($2 = 'admin' OR $2 = 'player'))
			OR (n.target_role = 'player' AND ($2 = 'admin' OR $2 = 'player'))
		))
	)
`

type NotificationInput struct {
	RecipientRole      string
	RecipientAccountID string
	SeasonID           string
	Category           string
	Type               string
	Priority           string
	Payload            interface{}
	Message            string
	Link               string
	Level              string
	AckRequired        bool
	ExpiresAt          *time.Time
	DedupKey           string
	DedupWindow        time.Duration
}

func NotificationCategories() []string {
	return append([]string{}, notificationCategoryList...)
}

func normalizeNotificationRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return ""
	}
	if role == "user" {
		return NotificationRolePlayer
	}
	return role
}

func notificationRoleForAccount(account *Account) string {
	if account == nil {
		return NotificationRolePlayer
	}
	role := normalizeRole(account.Role)
	switch role {
	case NotificationRoleAdmin:
		return NotificationRoleAdmin
	case NotificationRoleModerator:
		return NotificationRoleModerator
	default:
		return NotificationRolePlayer
	}
}

func normalizeNotificationCategory(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		return NotificationCategorySystem
	}
	for _, item := range notificationCategoryList {
		if category == item {
			return category
		}
	}
	return NotificationCategorySystem
}

func normalizeNotificationPriority(priority string) string {
	priority = strings.ToLower(strings.TrimSpace(priority))
	if priority == "" {
		return NotificationPriorityNormal
	}
	switch priority {
	case NotificationPriorityHigh, NotificationPriorityCritical:
		return priority
	default:
		return NotificationPriorityNormal
	}
}

func notificationLevelForPriority(priority string) string {
	switch priority {
	case NotificationPriorityHigh:
		return "warn"
	case NotificationPriorityCritical:
		return "urgent"
	default:
		return "info"
	}
}

func notificationRetentionWindow() time.Duration {
	value := strings.TrimSpace(os.Getenv("NOTIFICATION_RETENTION_HOURS"))
	if value == "" {
		return 48 * time.Hour
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 48 * time.Hour
	}
	return time.Duration(parsed) * time.Hour
}

func notificationExpiryForPriority(priority string) time.Duration {
	switch normalizeNotificationPriority(priority) {
	case NotificationPriorityCritical:
		return 14 * 24 * time.Hour
	case NotificationPriorityHigh:
		return 7 * 24 * time.Hour
	default:
		return notificationRetentionWindow()
	}
}

func createNotification(db *sql.DB, input NotificationInput) error {
	return insertNotification(db, input)
}

func emitNotification(db *sql.DB, input NotificationInput) {
	go func() {
		if err := insertNotification(db, input); err != nil {
			log.Println("notification emit failed:", err)
		}
	}()
}

func insertNotification(db *sql.DB, input NotificationInput) error {
	role := normalizeNotificationRole(input.RecipientRole)
	if role == "" {
		role = NotificationRolePlayer
	}
	category := normalizeNotificationCategory(input.Category)
	priority := normalizeNotificationPriority(input.Priority)
	ackRequired := input.AckRequired || priority == NotificationPriorityCritical
	level := strings.ToLower(strings.TrimSpace(input.Level))
	if level == "" {
		level = notificationLevelForPriority(priority)
	}
	link := strings.TrimSpace(input.Link)

	var payload []byte
	if input.Payload != nil {
		encoded, err := json.Marshal(input.Payload)
		if err == nil {
			payload = encoded
		}
	}

	now := time.Now().UTC()
	var expires sql.NullTime
	if input.ExpiresAt != nil {
		expires = sql.NullTime{Time: *input.ExpiresAt, Valid: true}
	} else {
		retention := now.Add(notificationExpiryForPriority(priority))
		expires = sql.NullTime{Time: retention, Valid: true}
	}

	if input.DedupKey != "" && input.DedupWindow > 0 {
		var existing int64
		err := db.QueryRow(`
			SELECT id
			FROM notifications
			WHERE dedupe_key = $1
				AND COALESCE(recipient_account_id, account_id, '') = $2
				AND COALESCE(recipient_role, target_role) = $3
				AND created_at > NOW() - ($4 * INTERVAL '1 second')
			LIMIT 1
		`, strings.TrimSpace(input.DedupKey), strings.TrimSpace(input.RecipientAccountID), role, int(input.DedupWindow.Seconds())).Scan(&existing)
		if err == nil {
			return nil
		}
		if err != sql.ErrNoRows {
			return err
		}
	}

	legacyRole := role
	if role == NotificationRolePlayer {
		legacyRole = "user"
	}

	_, err := db.Exec(`
		INSERT INTO notifications (
			target_role,
			account_id,
			recipient_role,
			recipient_account_id,
			season_id,
			category,
			type,
			priority,
			payload,
			message,
			level,
			link,
			created_at,
			expires_at,
			ack_required,
			dedupe_key
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), $13, $14, $15)
	`,
		legacyRole,
		nullableString(strings.TrimSpace(input.RecipientAccountID)),
		role,
		nullableString(strings.TrimSpace(input.RecipientAccountID)),
		nullableString(strings.TrimSpace(input.SeasonID)),
		category,
		strings.TrimSpace(input.Type),
		priority,
		payload,
		strings.TrimSpace(input.Message),
		level,
		link,
		expires,
		ackRequired,
		nullableString(strings.TrimSpace(input.DedupKey)),
	)
	return err
}

func fetchNotifications(db *sql.DB, accountID string, accountRole string, afterID int64, limit int, ascending bool) ([]NotificationItem, error) {
	if limit <= 0 {
		limit = 60
	}
	if limit > 200 {
		limit = 200
	}
	order := "DESC"
	if ascending {
		order = "ASC"
	}
	args := []interface{}{accountID, accountRole}
	whereAfter := ""
	limitIndex := 3
	if afterID > 0 {
		whereAfter = "AND n.id > $3"
		args = append(args, afterID)
		limitIndex = 4
	}
	args = append(args, limit)

	query := `
		SELECT
			n.id,
			n.message,
			n.level,
			n.link,
			n.created_at,
			n.expires_at,
			COALESCE(n.category, 'system') AS category,
			COALESCE(n.type, '') AS type,
			COALESCE(n.priority, 'normal') AS priority,
			n.payload,
			n.ack_required,
			(r.notification_id IS NOT NULL) AS is_read,
			(a.notification_id IS NOT NULL) AS is_acknowledged,
			a.acknowledged_at
		FROM notifications n
		LEFT JOIN notification_reads r
			ON r.notification_id = n.id AND r.account_id = $1
		LEFT JOIN notification_acks a
			ON a.notification_id = n.id AND a.account_id = $1
		LEFT JOIN notification_deletes d
			ON d.notification_id = n.id AND d.account_id = $1
		LEFT JOIN notification_settings s
			ON s.account_id = $1 AND s.category = COALESCE(n.category, 'system')
		WHERE (n.expires_at IS NULL OR n.expires_at > NOW())
			AND d.notification_id IS NULL
` + notificationAccessSQL + `
			AND (
				COALESCE(n.priority, 'normal') <> 'normal'
				OR COALESCE(s.enabled, true) = true
			)
			` + whereAfter + `
		ORDER BY n.id ` + order + `
		LIMIT $` + strconv.Itoa(limitIndex)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []NotificationItem{}
	for rows.Next() {
		var item NotificationItem
		var expires sql.NullTime
		var link sql.NullString
		var payload sql.NullString
		var ackedAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.Message,
			&item.Level,
			&link,
			&item.CreatedAt,
			&expires,
			&item.Category,
			&item.Type,
			&item.Priority,
			&payload,
			&item.AckRequired,
			&item.IsRead,
			&item.IsAcknowledged,
			&ackedAt,
		); err != nil {
			continue
		}
		if link.Valid {
			item.Link = link.String
		}
		if expires.Valid {
			item.ExpiresAt = &expires.Time
		}
		if payload.Valid {
			item.Payload = json.RawMessage(payload.String)
		}
		if ackedAt.Valid {
			item.AcknowledgedAt = &ackedAt.Time
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func pruneNotifications(db *sql.DB) {
	cutoff := time.Now().UTC().Add(-notificationRetentionWindow())
	if _, err := db.Exec(`
		DELETE FROM notifications
		WHERE (expires_at IS NOT NULL AND expires_at < NOW())
			OR (expires_at IS NULL AND created_at < $1)
	`, cutoff); err != nil {
		log.Println("notification prune failed:", err)
	}
	_, _ = db.Exec(`DELETE FROM notification_reads WHERE notification_id NOT IN (SELECT id FROM notifications)`)
	_, _ = db.Exec(`DELETE FROM notification_acks WHERE notification_id NOT IN (SELECT id FROM notifications)`)
	_, _ = db.Exec(`DELETE FROM notification_deletes WHERE notification_id NOT IN (SELECT id FROM notifications)`)
}

func startNotificationPruner(db *sql.DB) {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			pruneNotifications(db)
		}
	}()
}
