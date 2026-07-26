package globaldb

import (
	"database/sql"
	"fmt"

	"github.com/compozy/agh/internal/notifications"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

type notificationCursorScanner interface {
	Scan(dest ...any) error
}

func scanNotificationCursor(scanner notificationCursorScanner) (notifications.Cursor, error) {
	var (
		row             sqlcgen.NotificationCursor
		lastDeliveredAt sql.NullString
	)
	if err := scanner.Scan(
		&row.ConsumerID,
		&row.StreamName,
		&row.SubjectID,
		&row.LastSequence,
		&row.LastDeliveryID,
		&lastDeliveredAt,
		&row.LastError,
		&row.UpdatedAt,
	); err != nil {
		return notifications.Cursor{}, fmt.Errorf("store: scan notification cursor: %w", err)
	}
	row.LastDeliveredAt = lastDeliveredAt
	return notificationCursorFromGenerated(row)
}

func notificationCursorFromGenerated(row sqlcgen.NotificationCursor) (notifications.Cursor, error) {
	cursor := notifications.Cursor{
		Key: notifications.CursorKey{
			ConsumerID: row.ConsumerID,
			StreamName: row.StreamName,
			SubjectID:  row.SubjectID,
		},
		LastSequence:   row.LastSequence,
		LastDeliveryID: row.LastDeliveryID,
		LastError:      row.LastError,
	}
	if row.LastDeliveredAt.Valid {
		parsed, err := store.ParseTimestamp(row.LastDeliveredAt.String)
		if err != nil {
			return notifications.Cursor{}, fmt.Errorf("store: parse notification cursor delivery time: %w", err)
		}
		cursor.LastDeliveredAt = parsed
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return notifications.Cursor{}, fmt.Errorf("store: parse notification cursor update time: %w", err)
	}
	cursor.UpdatedAt = updatedAt
	return cursor, nil
}
