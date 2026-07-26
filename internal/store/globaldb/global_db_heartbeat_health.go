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

func (g *HeartbeatRepo) UpsertSessionHealth(
	ctx context.Context,
	health heartbeat.SessionHealth,
) (heartbeat.SessionHealth, error) {
	if err := g.checkReady(ctx, "upsert session health"); err != nil {
		return heartbeat.SessionHealth{}, err
	}
	normalized := health.Normalize()
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return heartbeat.SessionHealth{}, err
	}

	err := g.queries.UpsertSessionHealth(ctx, sqlcgen.UpsertSessionHealthParams{
		SessionID:           normalized.SessionID,
		WorkspaceID:         normalized.WorkspaceID,
		AgentName:           normalized.AgentName,
		State:               string(normalized.State),
		Health:              string(normalized.Health),
		ActivePrompt:        normalized.ActivePrompt,
		Attachable:          normalized.Attachable,
		EligibleForWake:     normalized.EligibleForWake,
		IneligibilityReason: nullableHeartbeatString(normalized.IneligibilityReason),
		LastActivityAt:      nullableHeartbeatTimestamp(normalized.LastActivityAt),
		LastPresenceAt:      nullableHeartbeatTimestamp(normalized.LastPresenceAt),
		LastError: nullableHeartbeatString(
			normalized.LastError,
		),
		UpdatedAt: store.FormatTimestamp(normalized.UpdatedAt),
	})
	if err != nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("store: upsert session health %q: %w", normalized.SessionID, err)
	}
	return normalized, nil
}

// GetSessionHealth returns metadata-only health for one session.
func (g *HeartbeatRepo) GetSessionHealth(ctx context.Context, sessionID string) (heartbeat.SessionHealth, error) {
	if err := g.checkReady(ctx, "get session health"); err != nil {
		return heartbeat.SessionHealth{}, err
	}
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return heartbeat.SessionHealth{}, fmt.Errorf("%w: session id is required", heartbeat.ErrInvalidSessionHealth)
	}

	row, err := g.queries.GetSessionHealth(ctx, trimmedID)
	if errors.Is(err, sql.ErrNoRows) {
		return heartbeat.SessionHealth{}, fmt.Errorf(
			"store: session health %q: %w",
			trimmedID,
			heartbeat.ErrSessionHealthNotFound,
		)
	}
	if err != nil {
		return heartbeat.SessionHealth{}, err
	}
	return heartbeatSessionHealthFromGenerated(row)
}

// ListSessionHealth lists metadata-only session health rows in newest-first order.
func (g *HeartbeatRepo) ListSessionHealth(
	ctx context.Context,
	query heartbeat.SessionHealthListQuery,
) (rowsOut []heartbeat.SessionHealth, err error) {
	if err := g.checkReady(ctx, "list session health"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// dynamic-sql: optional workspace/agent/session/state/health/wake filters and limit change the statement shape.
	sqlQuery := `SELECT session_id, workspace_id, agent_name, state, health, active_prompt, attachable,
			eligible_for_wake, ineligibility_reason, last_activity_at, last_presence_at,
			last_error, updated_at
		FROM session_health`
	where, args := store.BuildClauses(
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("agent_name", query.AgentName),
		store.StringClause("session_id", query.SessionID),
		store.StringClause("state", string(query.State)),
		store.StringClause("health", string(query.Health)),
	)
	if query.EligibleForWake != nil {
		where = append(where, "eligible_for_wake = ?")
		args = append(args, heartbeatBoolToInt(*query.EligibleForWake))
	}
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY updated_at DESC, session_id DESC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	result, err := querySessionHealthRows(ctx, g.db, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListSessionHealthRecoveryInputs returns persisted rows that restart recovery must recompute before wake.
func (g *HeartbeatRepo) ListSessionHealthRecoveryInputs(
	ctx context.Context,
	limit int,
) ([]heartbeat.SessionHealth, error) {
	if limit < 0 {
		return nil, fmt.Errorf("%w: invalid recovery limit %d", heartbeat.ErrInvalidSessionHealth, limit)
	}
	return g.ListSessionHealth(ctx, heartbeat.SessionHealthListQuery{Limit: limit})
}

// MarkSessionHealthStale marks stale persisted health rows as wake-ineligible without deleting authored policy.
func (g *HeartbeatRepo) MarkSessionHealthStale(
	ctx context.Context,
	cutoff time.Time,
	updatedAt time.Time,
) (int64, error) {
	if err := g.checkReady(ctx, "mark session health stale"); err != nil {
		return 0, err
	}
	if cutoff.IsZero() {
		return 0, fmt.Errorf("%w: stale cutoff is required", heartbeat.ErrInvalidSessionHealth)
	}
	if updatedAt.IsZero() {
		updatedAt = g.now()
	}
	affected, err := g.queries.MarkSessionHealthStale(ctx, sqlcgen.MarkSessionHealthStaleParams{
		Health:              string(heartbeat.SessionHealthStale),
		IneligibilityReason: nullableHeartbeatString(string(heartbeat.SessionHealthReasonStale)),
		UpdatedAt:           store.FormatTimestamp(updatedAt), Cutoff: nullableHeartbeatTimestamp(cutoff),
	})
	if err != nil {
		return 0, fmt.Errorf("store: mark session health stale: %w", err)
	}
	return affected, nil
}

// UpsertHeartbeatWakeState stores the latest per-session Heartbeat wake summary.
func querySessionHealthRows(
	ctx context.Context,
	db *sql.DB,
	sqlQuery string,
	args ...any,
) (healthRows []heartbeat.SessionHealth, err error) {
	rows, err := db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query session health: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close session health rows: %w", closeErr))
		}
	}()

	healthRows = make([]heartbeat.SessionHealth, 0)
	for rows.Next() {
		health, scanErr := scanSessionHealth(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		healthRows = append(healthRows, health)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate session health: %w", err)
	}
	return healthRows, nil
}
