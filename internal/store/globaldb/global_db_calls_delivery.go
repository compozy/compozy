package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/store"
)

func (g *CallRepo) ListPendingDeliveries(
	ctx context.Context,
	recipientSessionID string,
	limit int,
) ([]callspkg.DeliveryRecord, error) {
	if err := g.checkReady(ctx, "list pending call deliveries"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT delivery_id, kind, subject_id, recipient_session_id, owner_key,
		wake_event_id, state, reason, attempts, created_at
		FROM call_deliveries WHERE state = 'pending'`
	args := make([]any, 0, 2)
	if recipientSessionID = strings.TrimSpace(recipientSessionID); recipientSessionID != "" {
		query += ` AND recipient_session_id = ?`
		args = append(args, recipientSessionID)
	}
	query += ` ORDER BY created_at, delivery_id LIMIT ?`
	args = append(args, limit)
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list pending call deliveries: %w", err)
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	records := make([]callspkg.DeliveryRecord, 0)
	for rows.Next() {
		var record callspkg.DeliveryRecord
		var createdAt string
		if scanErr := rows.Scan(
			&record.DeliveryID, &record.Kind, &record.SubjectID, &record.RecipientSessionID,
			&record.OwnerKey, &record.WakeEventID, &record.State, &record.Reason,
			&record.Attempts, &createdAt,
		); scanErr != nil {
			return nil, fmt.Errorf("store: scan pending call delivery: %w", scanErr)
		}
		parsed, parseErr := store.ParseTimestamp(createdAt)
		if parseErr != nil {
			return nil, fmt.Errorf("store: parse call delivery created_at: %w", parseErr)
		}
		record.CreatedAt = parsed
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate pending call deliveries: %w", err)
	}
	return records, nil
}

func (g *CallRepo) RecordDelivery(
	ctx context.Context,
	update callspkg.DeliveryUpdate,
) (record callspkg.DeliveryRecord, err error) {
	if err := g.checkReady(ctx, "record call delivery"); err != nil {
		return callspkg.DeliveryRecord{}, err
	}
	if update.MaxAttempts <= 0 {
		return callspkg.DeliveryRecord{}, errors.New("store: delivery maximum attempts must be positive")
	}
	err = g.tasks.withTaskImmediateTransaction(ctx, "record call delivery", func(exec taskSQLExecutor) error {
		var currentState string
		if err := exec.QueryRowContext(ctx, `SELECT state FROM call_deliveries WHERE delivery_id = ?`,
			strings.TrimSpace(update.DeliveryID)).Scan(&currentState); err != nil {
			return fmt.Errorf("store: inspect call delivery %q: %w", update.DeliveryID, err)
		}
		if currentState != "pending" {
			return scanDeliveryRecord(exec.QueryRowContext(ctx, deliveryRecordSelectSQL, update.DeliveryID), &record)
		}
		state := update.State
		if state == "pending" {
			_, err := exec.ExecContext(ctx, `UPDATE call_deliveries
				SET state = CASE WHEN attempts + 1 >= ? THEN 'failed' ELSE 'pending' END,
				reason = ?, attempts = attempts + 1, updated_at = ?,
				delivered_at = CASE WHEN attempts + 1 >= ? THEN ? ELSE NULL END
				WHERE delivery_id = ? AND state = 'pending'`,
				update.MaxAttempts, strings.TrimSpace(update.Reason), store.FormatTimestamp(update.At),
				update.MaxAttempts, store.FormatTimestamp(update.At), update.DeliveryID,
			)
			if err != nil {
				return fmt.Errorf("store: record call delivery failure %q: %w", update.DeliveryID, err)
			}
			return scanDeliveryRecord(exec.QueryRowContext(ctx, deliveryRecordSelectSQL, update.DeliveryID), &record)
		}
		if state != "injected" && state != "woken" && state != "failed" {
			return fmt.Errorf("store: unsupported call delivery state %q", state)
		}
		deliveredAt := any(nil)
		if state == "injected" || state == "woken" || state == "failed" {
			deliveredAt = store.FormatTimestamp(update.At)
		}
		_, err := exec.ExecContext(ctx, `UPDATE call_deliveries
			SET state = ?, reason = ?, attempts = attempts + 1, updated_at = ?, delivered_at = ?
			WHERE delivery_id = ? AND state = 'pending'`, state, strings.TrimSpace(update.Reason),
			store.FormatTimestamp(update.At), deliveredAt, update.DeliveryID)
		if err != nil {
			return fmt.Errorf("store: update call delivery %q: %w", update.DeliveryID, err)
		}
		return scanDeliveryRecord(exec.QueryRowContext(ctx, deliveryRecordSelectSQL, update.DeliveryID), &record)
	})
	return record, err
}

const deliveryRecordSelectSQL = `SELECT delivery_id, kind, subject_id, recipient_session_id,
	owner_key, wake_event_id, state, reason, attempts, created_at
	FROM call_deliveries WHERE delivery_id = ?`

func scanDeliveryRecord(scanner rowScanner, record *callspkg.DeliveryRecord) error {
	var createdAt string
	if err := scanner.Scan(
		&record.DeliveryID, &record.Kind, &record.SubjectID, &record.RecipientSessionID,
		&record.OwnerKey, &record.WakeEventID, &record.State, &record.Reason,
		&record.Attempts, &createdAt,
	); err != nil {
		return fmt.Errorf("store: scan call delivery: %w", err)
	}
	parsed, err := store.ParseTimestamp(createdAt)
	if err != nil {
		return fmt.Errorf("store: parse call delivery created_at: %w", err)
	}
	record.CreatedAt = parsed
	return nil
}

func (g *CallRepo) ParkCallChild(
	ctx context.Context,
	sessionID string,
	parkedAt time.Time,
	idleExpiresAt time.Time,
) (bool, error) {
	if err := g.checkReady(ctx, "park call child"); err != nil {
		return false, err
	}
	result, err := g.db.ExecContext(ctx, `UPDATE sessions SET parked_at = ?, idle_expires_at = ?, updated_at = ?
		WHERE id = ? AND NOT EXISTS (
			SELECT 1 FROM calls WHERE child_session_id = sessions.id AND state IN ('queued', 'running')
		) AND NOT EXISTS (
			SELECT 1 FROM call_deliveries WHERE recipient_session_id = sessions.id AND state = 'pending'
		)`, store.FormatTimestamp(parkedAt), store.FormatTimestamp(idleExpiresAt),
		store.FormatTimestamp(parkedAt), strings.TrimSpace(sessionID))
	if err != nil {
		return false, fmt.Errorf("store: park call child %q: %w", sessionID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: inspect parked call child %q: %w", sessionID, err)
	}
	return affected == 1, nil
}

func (g *CallRepo) ClearCallChildIdleClock(ctx context.Context, sessionID string, at time.Time) error {
	if err := g.checkReady(ctx, "clear call child idle clock"); err != nil {
		return err
	}
	_, err := g.db.ExecContext(ctx, `UPDATE sessions SET parked_at = NULL, idle_expires_at = NULL, updated_at = ?
		WHERE id = ?`, store.FormatTimestamp(at), strings.TrimSpace(sessionID))
	if err != nil {
		return fmt.Errorf("store: clear call child idle clock %q: %w", sessionID, err)
	}
	return nil
}

func (g *CallRepo) GetCallPayload(ctx context.Context, workspaceID, ref string) ([]byte, error) {
	if err := g.checkReady(ctx, "get call payload"); err != nil {
		return nil, err
	}
	var payload []byte
	var byteSize int64
	ref = strings.TrimSpace(ref)
	err := g.db.QueryRowContext(ctx, `SELECT bytes, byte_size FROM payload_blobs WHERE workspace_id = ? AND ref = ?`,
		strings.TrimSpace(workspaceID), ref).Scan(&payload, &byteSize)
	if err != nil {
		return nil, fmt.Errorf("store: get call payload %q: %w", ref, err)
	}
	if int64(len(payload)) != byteSize || contracts.OutputRefForPayload(json.RawMessage(payload)) != ref {
		return nil, fmt.Errorf("store: call payload %q failed digest verification", ref)
	}
	return append([]byte(nil), payload...), nil
}

func (g *CallRepo) FailPendingDeliveriesForRecipient(
	ctx context.Context,
	sessionID string,
	reason string,
	at time.Time,
) error {
	if err := g.checkReady(ctx, "fail recipient call deliveries"); err != nil {
		return err
	}
	_, err := g.db.ExecContext(ctx, `UPDATE call_deliveries
		SET state = 'failed', reason = ?, updated_at = ?, delivered_at = ?
		WHERE recipient_session_id = ? AND state = 'pending' AND kind = 'message'`,
		strings.TrimSpace(reason), store.FormatTimestamp(at), store.FormatTimestamp(at),
		strings.TrimSpace(sessionID))
	if err != nil {
		return fmt.Errorf("store: fail pending deliveries for recipient %q: %w", sessionID, err)
	}
	return nil
}

func (g *CallRepo) FinalizeReapedSession(
	ctx context.Context,
	sessionID string,
	reason string,
	at time.Time,
) error {
	if err := g.checkReady(ctx, "finalize reaped call session"); err != nil {
		return err
	}
	_, err := g.db.ExecContext(ctx, `UPDATE sessions
		SET parked_at = NULL, idle_expires_at = NULL, draining_at = NULL, state = 'stopped',
		stop_reason = CASE WHEN ? = 'ttl_expired' THEN 'timeout' ELSE 'user_canceled' END,
		stop_detail = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(reason), strings.TrimSpace(reason),
		store.FormatTimestamp(at), strings.TrimSpace(sessionID))
	if err != nil {
		return fmt.Errorf("store: finalize reaped call session %q: %w", sessionID, err)
	}
	return nil
}

func (g *CallRepo) FenceSessionReap(ctx context.Context, sessionID string, at time.Time) (bool, error) {
	if err := g.checkReady(ctx, "fence call session reap"); err != nil {
		return false, err
	}
	result, err := g.db.ExecContext(ctx, `UPDATE sessions SET draining_at = ?, updated_at = ?
		WHERE id = ?
		AND NOT EXISTS (SELECT 1 FROM operator_caller_sessions operator WHERE operator.session_id = sessions.id)
		AND NOT EXISTS (SELECT 1 FROM calls WHERE (child_session_id = sessions.id OR parent_session_id = sessions.id)
			AND state IN ('queued', 'running'))`,
		store.FormatTimestamp(at), store.FormatTimestamp(at), strings.TrimSpace(sessionID))
	if err != nil {
		return false, fmt.Errorf("store: fence call session reap %q: %w", sessionID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: inspect call session reap fence %q: %w", sessionID, err)
	}
	return affected == 1, nil
}
