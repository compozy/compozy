package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func (g *HeartbeatRepo) UpsertHeartbeatWakeState(
	ctx context.Context,
	state heartbeat.WakeState,
) (heartbeat.WakeState, error) {
	if err := g.checkReady(ctx, "upsert heartbeat wake state"); err != nil {
		return heartbeat.WakeState{}, err
	}
	normalized := state.Normalize()
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return heartbeat.WakeState{}, err
	}

	if err := execHeartbeatWakeStateUpsert(ctx, g.db, normalized); err != nil {
		return heartbeat.WakeState{}, fmt.Errorf("store: upsert heartbeat wake state %q: %w", normalized.SessionID, err)
	}
	return normalized, nil
}

// GetHeartbeatWakeState returns one per-session Heartbeat wake summary.
func (g *HeartbeatRepo) GetHeartbeatWakeState(
	ctx context.Context,
	workspaceID string,
	agentName string,
	sessionID string,
) (heartbeat.WakeState, error) {
	if err := g.checkReady(ctx, "get heartbeat wake state"); err != nil {
		return heartbeat.WakeState{}, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	agentName = strings.TrimSpace(agentName)
	sessionID = strings.TrimSpace(sessionID)
	if workspaceID == "" || agentName == "" || sessionID == "" {
		return heartbeat.WakeState{}, fmt.Errorf(
			"%w: workspace id, agent name, and session id are required",
			heartbeat.ErrInvalidWakeState,
		)
	}

	row, err := g.queries.GetHeartbeatWakeState(ctx, sqlcgen.GetHeartbeatWakeStateParams{
		WorkspaceID: workspaceID, AgentName: agentName, SessionID: sessionID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return heartbeat.WakeState{}, fmt.Errorf(
			"store: heartbeat wake state %q: %w",
			sessionID,
			heartbeat.ErrWakeStateNotFound,
		)
	}
	if err != nil {
		return heartbeat.WakeState{}, err
	}
	return heartbeatWakeStateFromGenerated(row)
}

// ListHeartbeatWakeState lists Heartbeat wake state rows in newest-first order.
func (g *HeartbeatRepo) ListHeartbeatWakeState(
	ctx context.Context,
	query heartbeat.WakeStateListQuery,
) (states []heartbeat.WakeState, err error) {
	if err := g.checkReady(ctx, "list heartbeat wake state"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// dynamic-sql: optional workspace/agent/session filters and the caller limit change the statement shape.
	sqlQuery := `SELECT workspace_id, agent_name, session_id, policy_snapshot_id, last_wake_at, next_allowed_at,
			coalesced_count, last_result, last_reason, updated_at
		FROM agent_heartbeat_wake_state`
	where, args := store.BuildClauses(
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("agent_name", query.AgentName),
		store.StringClause("session_id", query.SessionID),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY updated_at DESC, session_id DESC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query heartbeat wake state: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close heartbeat wake state rows: %w", closeErr))
		}
	}()

	states = make([]heartbeat.WakeState, 0)
	for rows.Next() {
		state, scanErr := scanHeartbeatWakeState(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate heartbeat wake state: %w", err)
	}
	return states, nil
}

// RecordHeartbeatWakeDecision appends the audit row and updates cooldown state atomically.
func (g *HeartbeatRepo) RecordHeartbeatWakeDecision(
	ctx context.Context,
	event heartbeat.WakeEvent,
	state heartbeat.WakeState,
) (outEvent heartbeat.WakeEvent, outState heartbeat.WakeState, err error) {
	if err := g.checkReady(ctx, "record heartbeat wake decision"); err != nil {
		return heartbeat.WakeEvent{}, heartbeat.WakeState{}, err
	}
	normalizedEvent := event.Normalize()
	if normalizedEvent.ID == "" {
		normalizedEvent.ID = store.NewID("hwe")
	}
	if normalizedEvent.CreatedAt.IsZero() {
		normalizedEvent.CreatedAt = g.now()
	}
	if err := normalizedEvent.Validate(); err != nil {
		return heartbeat.WakeEvent{}, heartbeat.WakeState{}, err
	}
	normalizedState := state.Normalize()
	if normalizedState.UpdatedAt.IsZero() {
		normalizedState.UpdatedAt = g.now()
	}
	if err := normalizedState.Validate(); err != nil {
		return heartbeat.WakeEvent{}, heartbeat.WakeState{}, err
	}

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return heartbeat.WakeEvent{}, heartbeat.WakeState{}, fmt.Errorf(
			"store: begin heartbeat wake decision transaction: %w",
			err,
		)
	}
	finished := false
	defer func() {
		if !finished {
			joinCleanupError(&err, rollbackTx(tx, "heartbeat wake decision"))
		}
	}()

	if err := execHeartbeatWakeEventInsert(ctx, tx, normalizedEvent); err != nil {
		return heartbeat.WakeEvent{}, heartbeat.WakeState{}, fmt.Errorf(
			"store: append heartbeat wake event %q: %w",
			normalizedEvent.ID,
			err,
		)
	}
	if err := execHeartbeatWakeStateUpsert(ctx, tx, normalizedState); err != nil {
		return heartbeat.WakeEvent{}, heartbeat.WakeState{}, fmt.Errorf(
			"store: upsert heartbeat wake state %q: %w",
			normalizedState.SessionID,
			err,
		)
	}
	if err := tx.Commit(); err != nil {
		return heartbeat.WakeEvent{}, heartbeat.WakeState{}, fmt.Errorf(
			"store: commit heartbeat wake decision transaction: %w",
			err,
		)
	}
	finished = true
	return normalizedEvent, normalizedState, nil
}

// AppendHeartbeatWakeEvent appends one retained Heartbeat wake audit row.
func (g *HeartbeatRepo) AppendHeartbeatWakeEvent(
	ctx context.Context,
	event heartbeat.WakeEvent,
) (heartbeat.WakeEvent, error) {
	if err := g.checkReady(ctx, "append heartbeat wake event"); err != nil {
		return heartbeat.WakeEvent{}, err
	}
	normalized := event.Normalize()
	if normalized.ID == "" {
		normalized.ID = store.NewID("hwe")
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return heartbeat.WakeEvent{}, err
	}

	if err := execHeartbeatWakeEventInsert(ctx, g.db, normalized); err != nil {
		return heartbeat.WakeEvent{}, fmt.Errorf("store: append heartbeat wake event %q: %w", normalized.ID, err)
	}
	return normalized, nil
}

func execHeartbeatWakeStateUpsert(ctx context.Context, exec globalSQLExecutor, state heartbeat.WakeState) error {
	return sqlcgen.New(exec).UpsertHeartbeatWakeState(ctx, sqlcgen.UpsertHeartbeatWakeStateParams{
		WorkspaceID:      state.WorkspaceID,
		AgentName:        state.AgentName,
		SessionID:        state.SessionID,
		PolicySnapshotID: nullableHeartbeatString(state.PolicySnapshotID),
		LastWakeAt: nullableHeartbeatTimestamp(
			state.LastWakeAt,
		),
		NextAllowedAt:  nullableHeartbeatTimestamp(state.NextAllowedAt),
		CoalescedCount: int64(state.CoalescedCount),
		LastResult:     string(state.LastResult),
		LastReason: nullableHeartbeatString(
			string(state.LastReason),
		),
		UpdatedAt: store.FormatTimestamp(state.UpdatedAt),
	})
}

func execHeartbeatWakeEventInsert(ctx context.Context, exec globalSQLExecutor, event heartbeat.WakeEvent) error {
	return sqlcgen.New(exec).InsertHeartbeatWakeEvent(ctx, sqlcgen.InsertHeartbeatWakeEventParams{
		ID:          event.ID,
		WorkspaceID: event.WorkspaceID,
		AgentName:   event.AgentName,
		SessionID: nullableHeartbeatString(
			event.SessionID,
		),
		PolicySnapshotID:  nullableHeartbeatString(event.PolicySnapshotID),
		Source:            string(event.Source),
		Result:            string(event.Result),
		Reason:            string(event.Reason),
		SyntheticPromptID: nullableHeartbeatString(event.SyntheticPromptID),
		CreatedAt:         store.FormatTimestamp(event.CreatedAt),
		ExpiresAt:         store.FormatTimestamp(event.ExpiresAt),
	})
}

// GetHeartbeatWakeEvent returns one retained Heartbeat wake audit row.
func (g *HeartbeatRepo) GetHeartbeatWakeEvent(ctx context.Context, id string) (heartbeat.WakeEvent, error) {
	if err := g.checkReady(ctx, "get heartbeat wake event"); err != nil {
		return heartbeat.WakeEvent{}, err
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return heartbeat.WakeEvent{}, fmt.Errorf("%w: id is required", heartbeat.ErrInvalidWakeEvent)
	}

	row, err := g.queries.GetHeartbeatWakeEvent(ctx, trimmedID)
	if errors.Is(err, sql.ErrNoRows) {
		return heartbeat.WakeEvent{}, fmt.Errorf(
			"store: heartbeat wake event %q: %w",
			trimmedID,
			heartbeat.ErrWakeEventNotFound,
		)
	}
	if err != nil {
		return heartbeat.WakeEvent{}, err
	}
	return heartbeatWakeEventFromGenerated(row)
}

// ListHeartbeatWakeEvents lists retained Heartbeat wake audit rows in newest-first order.
func (g *HeartbeatRepo) ListHeartbeatWakeEvents(
	ctx context.Context,
	query heartbeat.WakeEventListQuery,
) (events []heartbeat.WakeEvent, err error) {
	if err := g.checkReady(ctx, "list heartbeat wake events"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// dynamic-sql: optional workspace/agent/session/source/result/reason filters and limit change the statement shape.
	sqlQuery := `SELECT id, workspace_id, agent_name, session_id, policy_snapshot_id, source, result, reason,
			synthetic_prompt_id, created_at, expires_at
		FROM agent_heartbeat_wake_events`
	where, args := store.BuildClauses(
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("agent_name", query.AgentName),
		store.StringClause("session_id", query.SessionID),
		store.StringClause("source", string(query.Source)),
		store.StringClause("result", string(query.Result)),
		store.StringClause("reason", string(query.Reason)),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += heartbeatOrderByCreatedDesc
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query heartbeat wake events: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close heartbeat wake event rows: %w", closeErr))
		}
	}()

	events = make([]heartbeat.WakeEvent, 0)
	for rows.Next() {
		event, scanErr := scanHeartbeatWakeEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate heartbeat wake events: %w", err)
	}
	return events, nil
}

// SweepHeartbeatWakeEvents deletes expired wake audit rows in one bounded batch.
func (g *HeartbeatRepo) SweepHeartbeatWakeEvents(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if err := g.checkReady(ctx, "sweep heartbeat wake events"); err != nil {
		return 0, err
	}
	if cutoff.IsZero() {
		return 0, fmt.Errorf("%w: retention cutoff is required", heartbeat.ErrInvalidWakeEvent)
	}
	if limit <= 0 {
		return 0, fmt.Errorf("%w: retention limit must be positive", heartbeat.ErrInvalidWakeEvent)
	}
	affected, err := g.queries.SweepHeartbeatWakeEvents(ctx, sqlcgen.SweepHeartbeatWakeEventsParams{
		Cutoff: store.FormatTimestamp(cutoff), RowLimit: int64(limit),
	})
	if err != nil {
		return 0, fmt.Errorf("store: sweep heartbeat wake events: %w", err)
	}
	return affected, nil
}
