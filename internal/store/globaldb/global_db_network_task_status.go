package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	networkTaskConversationAvailable       = "available"
	networkTaskConversationNetworkDisabled = "network_disabled"
)

func persistNetworkTaskStatusProjectionWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	event taskpkg.Event,
) error {
	eventType := taskpkg.StatusProjectionEventType(event.EventType)
	if eventType == "" {
		return nil
	}
	origin, found, err := networkTaskThreadOriginWithExecutor(ctx, exec, event.TaskID)
	if err != nil || !found {
		return err
	}
	taskRecord, err := (&TaskRepo{}).getTaskWithExecutor(ctx, exec, event.TaskID)
	if err != nil {
		return fmt.Errorf("store: load task for network status projection: %w", err)
	}
	projection := store.NetworkTaskStatusProjectionPayload{
		EventID: event.ID, EventType: eventType, TaskID: event.TaskID,
		RunID: strings.TrimSpace(event.RunID), TaskStatus: string(taskRecord.Status.Normalize()),
		NetworkParticipation: participation.LocalSpec(), ProjectedAt: store.FormatTimestamp(event.Timestamp),
		ConversationAvailability: networkTaskConversationNetworkDisabled,
	}
	if projection.RunID != "" {
		run, loadErr := (&TaskRepo{}).getTaskRunWithExecutor(ctx, exec, projection.RunID)
		if loadErr != nil {
			return fmt.Errorf("store: load task run for network status projection: %w", loadErr)
		}
		projection.RunStatus = run.Status.Normalize().String()
		projection.NetworkParticipation = run.NetworkSpecSnapshot()
	}
	availability, err := sqlcgen.New(exec).GetNetworkAvailability(ctx)
	if err != nil {
		return fmt.Errorf("store: load network availability for task status projection: %w", err)
	}
	if availability.Enabled != 0 {
		projection.ConversationAvailable = true
		projection.ConversationAvailability = networkTaskConversationAvailable
	}
	payload, err := json.Marshal(projection)
	if err != nil {
		return fmt.Errorf("store: marshal network task status projection: %w", err)
	}
	normalized := normalizeNetworkTaskStatusPublication(store.NetworkTaskStatusPublication{
		EventID: event.ID, EventType: eventType, TaskID: event.TaskID, RunID: event.RunID,
		WorkspaceID: origin.WorkspaceID, Channel: origin.Channel, ThreadID: origin.ThreadID,
		ProjectionJSON: payload, ProjectedAt: event.Timestamp,
	})
	if err := normalized.Validate(); err != nil {
		return fmt.Errorf("store: validate network task status publication: %w", err)
	}
	const statement = `INSERT INTO network_task_status_projections (
		event_id, recipient_session_id, workspace_id, channel, thread_id,
		task_id, run_id, event_type, projection_json, projected_at
	)
	SELECT ?, subscription.session_id, ?, ?, ?, ?, ?, ?, ?, ?
	FROM network_subscriptions AS subscription
	WHERE subscription.workspace_id = ?
	  AND subscription.channel = ?
	  AND subscription.thread_id = ?
	  AND subscription.mode = 'full'
	ON CONFLICT(event_id, recipient_session_id) DO NOTHING`
	if _, err := exec.ExecContext(
		ctx,
		statement,
		normalized.EventID,
		normalized.WorkspaceID,
		normalized.Channel,
		normalized.ThreadID,
		normalized.TaskID,
		normalized.RunID,
		normalized.EventType,
		string(normalized.ProjectionJSON),
		store.FormatTimestamp(normalized.ProjectedAt),
		normalized.WorkspaceID,
		normalized.Channel,
		normalized.ThreadID,
	); err != nil {
		return fmt.Errorf("store: persist network task status projection: %w", err)
	}
	return nil
}

func networkTaskThreadOriginWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
) (store.NetworkTaskThreadOrigin, bool, error) {
	const statement = `SELECT task_id, workspace_id, channel, thread_id,
		origin_message_id, digest, source_message_ids_json, created_at, updated_at
		FROM network_task_thread_origins
		WHERE task_id = ?`
	origin, err := scanNetworkTaskThreadOrigin(exec.QueryRowContext(ctx, statement, strings.TrimSpace(taskID)))
	if errors.Is(err, sql.ErrNoRows) {
		return store.NetworkTaskThreadOrigin{}, false, nil
	}
	if err != nil {
		return store.NetworkTaskThreadOrigin{}, false, fmt.Errorf(
			"store: load network task thread origin for projection: %w",
			err,
		)
	}
	return origin, true, nil
}

// ListNetworkTaskStatusProjections returns bounded recipient-scoped durable projections.
func (g *NetworkRepo) ListNetworkTaskStatusProjections(
	ctx context.Context,
	query store.NetworkTaskStatusProjectionQuery,
) (projections []store.NetworkTaskStatusProjection, err error) {
	if err := g.checkReady(ctx, "list network task status projections"); err != nil {
		return nil, err
	}
	normalized := store.NetworkTaskStatusProjectionQuery{
		WorkspaceID:        strings.TrimSpace(query.WorkspaceID),
		RecipientSessionID: strings.TrimSpace(query.RecipientSessionID),
		TaskID:             strings.TrimSpace(query.TaskID),
		Limit:              query.Limit,
	}
	if err := normalized.Validate(); err != nil {
		return nil, fmt.Errorf("store: validate network task status projection query: %w", err)
	}
	statement := `SELECT event_id, recipient_session_id, workspace_id, channel, thread_id,
		task_id, run_id, event_type, projection_json, projected_at
		FROM network_task_status_projections`
	where, args := store.BuildClauses(
		store.StringClause("workspace_id", normalized.WorkspaceID),
		store.StringClause("recipient_session_id", normalized.RecipientSessionID),
		store.StringClause("task_id", normalized.TaskID),
	)
	statement = store.AppendWhere(statement, where)
	statement += " ORDER BY projected_at DESC, event_id ASC, recipient_session_id ASC"
	statement, args = store.AppendLimit(statement, args, normalized.Limit)

	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query network task status projections: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close network task status projection rows: %w", closeErr)
			err = errors.Join(err, closeErr)
		}
	}()

	projections = make([]store.NetworkTaskStatusProjection, 0)
	for rows.Next() {
		projection, scanErr := scanNetworkTaskStatusProjection(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		projections = append(projections, projection)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate network task status projections: %w", err)
	}
	return projections, nil
}

func normalizeNetworkTaskStatusPublication(
	publication store.NetworkTaskStatusPublication,
) store.NetworkTaskStatusPublication {
	return store.NetworkTaskStatusPublication{
		EventID:        strings.TrimSpace(publication.EventID),
		EventType:      strings.TrimSpace(publication.EventType),
		TaskID:         strings.TrimSpace(publication.TaskID),
		RunID:          strings.TrimSpace(publication.RunID),
		WorkspaceID:    strings.TrimSpace(publication.WorkspaceID),
		Channel:        strings.TrimSpace(publication.Channel),
		ThreadID:       strings.TrimSpace(publication.ThreadID),
		ProjectionJSON: json.RawMessage(strings.TrimSpace(string(publication.ProjectionJSON))),
		ProjectedAt:    publication.ProjectedAt.UTC(),
	}
}

func scanNetworkTaskStatusProjection(scanner rowScanner) (store.NetworkTaskStatusProjection, error) {
	var projection store.NetworkTaskStatusProjection
	var projectionRaw, projectedAtRaw string
	if err := scanner.Scan(
		&projection.EventID,
		&projection.RecipientSessionID,
		&projection.WorkspaceID,
		&projection.Channel,
		&projection.ThreadID,
		&projection.TaskID,
		&projection.RunID,
		&projection.EventType,
		&projectionRaw,
		&projectedAtRaw,
	); err != nil {
		return store.NetworkTaskStatusProjection{}, fmt.Errorf(
			"store: scan network task status projection: %w",
			err,
		)
	}
	projectedAt, err := store.ParseTimestamp(projectedAtRaw)
	if err != nil {
		return store.NetworkTaskStatusProjection{}, fmt.Errorf(
			"store: parse network task status projection projected_at: %w",
			err,
		)
	}
	projection.ProjectionJSON = json.RawMessage(projectionRaw)
	projection.ProjectedAt = projectedAt
	return projection, nil
}
