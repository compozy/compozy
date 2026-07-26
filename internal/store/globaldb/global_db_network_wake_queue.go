package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

var _ store.NetworkWakeQueueStore = (*NetworkRepo)(nil)

// ListQueuedNetworkWakes returns the durable queue projection used at boot and after lost notifications.
func (g *NetworkRepo) ListQueuedNetworkWakes(
	ctx context.Context,
	limit int,
) (notifications []store.CommittedNetworkNotification, err error) {
	if err := g.checkReady(ctx, "list queued network wakes"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT run.workspace_id, run.network_target_session_id, run.id,
			COALESCE(MAX(disposition.acceptance_seq), 0), wake.coalesce_until
		 FROM task_runs AS run
		 JOIN network_live_wakes AS wake
			ON wake.workspace_id = run.workspace_id
			AND wake.task_run_id = run.id
		 LEFT JOIN network_wake_sources AS source
			ON source.workspace_id = wake.workspace_id
			AND source.wake_id = wake.wake_id
		 LEFT JOIN network_message_dispositions AS disposition
			ON disposition.workspace_id = source.workspace_id
			AND disposition.message_id = source.envelope_id
			AND disposition.recipient_session_id = run.network_target_session_id
		 WHERE run.run_kind = ? AND run.status = ? AND wake.state = 'open'
		 GROUP BY run.workspace_id, run.network_target_session_id, run.id,
			wake.coalesce_until, run.queued_at
		 ORDER BY run.queued_at ASC, run.id ASC
		 LIMIT ?`,
		taskpkg.RunKindNetworkWake.String(),
		taskpkg.TaskRunStatusQueued.String(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("store: query queued network wakes: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close queued network wakes rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, closeErr)
				return
			}
			err = closeErr
		}
	}()
	notifications = make([]store.CommittedNetworkNotification, 0)
	for rows.Next() {
		var notification store.CommittedNetworkNotification
		var readyAtRaw string
		if err := rows.Scan(
			&notification.WorkspaceID,
			&notification.RecipientSessionID,
			&notification.TaskRunID,
			&notification.AcceptanceSeq,
			&readyAtRaw,
		); err != nil {
			return nil, fmt.Errorf("store: scan queued network wake: %w", err)
		}
		notification.ReadyAt, err = store.ParseTimestamp(readyAtRaw)
		if err != nil {
			return nil, fmt.Errorf("store: parse queued network wake ready_at: %w", err)
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate queued network wakes: %w", err)
	}
	return notifications, nil
}

// LoadNetworkWake loads one reservation and its causally ordered source messages.
func (g *NetworkRepo) LoadNetworkWake(
	ctx context.Context,
	workspaceID string,
	wakeID string,
) (store.WakeReservation, []store.NetworkMessageEntry, error) {
	if err := g.checkReady(ctx, "load network wake"); err != nil {
		return store.WakeReservation{}, nil, err
	}
	targetWakeID := strings.TrimSpace(wakeID)
	targetWorkspaceID := strings.TrimSpace(workspaceID)
	if targetWorkspaceID == "" {
		return store.WakeReservation{}, nil, fmt.Errorf("store: network wake workspace_id is required")
	}
	if targetWakeID == "" {
		return store.WakeReservation{}, nil, fmt.Errorf("store: network wake id is required")
	}
	var reservation store.WakeReservation
	var channel string
	var reservedWallMS int64
	var coalesceUntilRaw string
	err := g.db.QueryRowContext(
		ctx,
		`SELECT wake_id, task_run_id, owner_key, workspace_id, channel,
			reserved_wall_ms, coalesce_until
		 FROM network_live_wakes
		 WHERE workspace_id = ? AND wake_id = ?`,
		targetWorkspaceID,
		targetWakeID,
	).Scan(
		&reservation.WakeID,
		&reservation.TaskRunID,
		&reservation.OwnerKey,
		&reservation.WorkspaceID,
		&channel,
		&reservedWallMS,
		&coalesceUntilRaw,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.WakeReservation{}, nil, err
		}
		return store.WakeReservation{}, nil, fmt.Errorf("store: query network wake: %w", err)
	}
	reservation.CoalesceUntil, err = store.ParseTimestamp(coalesceUntilRaw)
	if err != nil {
		return store.WakeReservation{}, nil, fmt.Errorf("store: parse network wake coalesce_until: %w", err)
	}
	reservation.ReservedWallTime = (time.Duration(reservedWallMS) * time.Millisecond).String()
	envelopeIDs, err := g.listNetworkWakeSourceIDs(ctx, targetWorkspaceID, targetWakeID)
	if err != nil {
		return store.WakeReservation{}, nil, err
	}
	reservation.EnvelopeIDs = append([]string(nil), envelopeIDs...)
	messages := make([]store.NetworkMessageEntry, 0, len(envelopeIDs))
	for _, envelopeID := range envelopeIDs {
		entries, listErr := g.ListNetworkMessages(ctx, store.NetworkMessageQuery{
			WorkspaceID: targetWorkspaceID,
			Channel:     channel,
			MessageID:   envelopeID,
			Limit:       1,
		})
		if listErr != nil {
			return store.WakeReservation{}, nil, listErr
		}
		if len(entries) != 1 {
			return store.WakeReservation{}, nil, fmt.Errorf(
				"store: network wake source %q does not resolve to one message",
				envelopeID,
			)
		}
		messages = append(messages, entries[0])
	}
	return reservation, messages, nil
}

func (g *NetworkRepo) listNetworkWakeSourceIDs(
	ctx context.Context,
	workspaceID string,
	wakeID string,
) (ids []string, err error) {
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT source.envelope_id
		 FROM network_wake_sources AS source
		 JOIN network_timeline_log AS message
			ON message.workspace_id = source.workspace_id
			AND message.message_id = source.envelope_id
		 WHERE source.workspace_id = ? AND source.wake_id = ?
		 ORDER BY message.sequence ASC`,
		workspaceID,
		wakeID,
	)
	if err != nil {
		return nil, fmt.Errorf("store: query network wake sources: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close network wake source rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, closeErr)
				return
			}
			err = closeErr
		}
	}()
	ids = make([]string, 0)
	for rows.Next() {
		var envelopeID string
		if err := rows.Scan(&envelopeID); err != nil {
			return nil, fmt.Errorf("store: scan network wake source: %w", err)
		}
		ids = append(ids, envelopeID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate network wake sources: %w", err)
	}
	return ids, nil
}
