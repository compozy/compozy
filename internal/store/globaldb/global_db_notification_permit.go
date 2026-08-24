package globaldb

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/store"
)

var _ notifications.DeliveryPermitStore = (*NotificationRepo)(nil)

func (n *NotificationRepo) AcquireDeliveryPermit(
	ctx context.Context,
	permit notifications.DeliveryPermit,
) error {
	if err := n.checkReady(ctx, "acquire notification delivery permit"); err != nil {
		return err
	}
	normalized, err := permit.Normalize(n.now())
	if err != nil {
		return err
	}
	err = store.ExecuteWrite(ctx, n.db, func(writeCtx context.Context, tx *store.WriteTx) error {
		var active int
		if err := tx.QueryRowContext(
			writeCtx,
			`SELECT COUNT(*) FROM profiles
			 WHERE id = ? AND state = 'active'
			   AND NOT EXISTS (
				SELECT 1 FROM profile_lifecycle_ops
				WHERE profile_id = profiles.id AND status <> 'done'
			   )`,
			normalized.Key.ProfileID,
		).Scan(&active); err != nil {
			return fmt.Errorf("store: check notification permit owner: %w", err)
		}
		if active != 1 {
			return fmt.Errorf("profile_unavailable: notification delivery owner is not active")
		}
		_, err := tx.ExecContext(
			writeCtx,
			`INSERT INTO notification_delivery_permits
			 (scope_kind, profile_id, workspace_id, consumer_id, stream_name, subject_id, delivery_id, acquired_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(scope_kind, profile_id, workspace_id, consumer_id, stream_name, subject_id, delivery_id)
			 DO NOTHING`,
			string(normalized.Key.Scope.Kind),
			normalized.Key.ProfileID,
			normalized.Key.Scope.WorkspaceID,
			normalized.Key.ConsumerID,
			normalized.Key.StreamName,
			normalized.Key.SubjectID,
			normalized.DeliveryID,
			store.FormatTimestamp(normalized.AcquiredAt),
		)
		if err != nil {
			return fmt.Errorf("store: insert notification delivery permit: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("store: acquire notification delivery permit: %w", err)
	}
	return nil
}

// ListDeliveryPermits returns durable sends that have not advanced their cursor.
func (n *NotificationRepo) ListDeliveryPermits(
	ctx context.Context,
) (_ []notifications.DeliveryPermit, err error) {
	if err := n.checkReady(ctx, "list notification delivery permits"); err != nil {
		return nil, err
	}
	rows, err := n.db.QueryContext(ctx, `
		SELECT scope_kind, profile_id, workspace_id, consumer_id, stream_name,
		       subject_id, delivery_id, acquired_at
		FROM notification_delivery_permits
		ORDER BY acquired_at, profile_id, consumer_id, stream_name, subject_id, delivery_id`)
	if err != nil {
		return nil, fmt.Errorf("store: list notification delivery permits: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close notification delivery permit rows: %w", closeErr))
		}
	}()
	permits := make([]notifications.DeliveryPermit, 0)
	for rows.Next() {
		var permit notifications.DeliveryPermit
		var scopeKind string
		var acquiredAt string
		if scanErr := rows.Scan(
			&scopeKind,
			&permit.Key.ProfileID,
			&permit.Key.Scope.WorkspaceID,
			&permit.Key.ConsumerID,
			&permit.Key.StreamName,
			&permit.Key.SubjectID,
			&permit.DeliveryID,
			&acquiredAt,
		); scanErr != nil {
			return nil, fmt.Errorf("store: scan notification delivery permit: %w", scanErr)
		}
		permit.Key.Scope.Kind = notifications.ScopeKind(scopeKind)
		permit.AcquiredAt, err = store.ParseTimestamp(acquiredAt)
		if err != nil {
			return nil, fmt.Errorf("store: parse notification delivery permit acquisition: %w", err)
		}
		permits = append(permits, permit)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("store: iterate notification delivery permits: %w", rowsErr)
	}
	return permits, nil
}

var _ notifications.DeliveryPermitReader = (*NotificationRepo)(nil)

func clearNotificationDeliveryPermit(
	ctx context.Context,
	tx *store.WriteTx,
	update notifications.AdvanceCursor,
) error {
	if update.DeliveryID == "" {
		return nil
	}
	_, err := tx.ExecContext(
		ctx,
		`DELETE FROM notification_delivery_permits
		 WHERE scope_kind = ? AND profile_id = ? AND workspace_id = ?
		   AND consumer_id = ? AND stream_name = ? AND subject_id = ? AND delivery_id = ?`,
		string(update.Key.Scope.Kind),
		update.Key.ProfileID,
		update.Key.Scope.WorkspaceID,
		update.Key.ConsumerID,
		update.Key.StreamName,
		update.Key.SubjectID,
		update.DeliveryID,
	)
	if err != nil {
		return fmt.Errorf("store: clear notification delivery permit: %w", err)
	}
	return nil
}
