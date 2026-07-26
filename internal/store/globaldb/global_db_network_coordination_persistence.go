package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (g *WorkspaceRepo) compareAndSetCoordinationWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	ref workspacepkg.CoordinationRef,
	enabled bool,
	expectedRevision int64,
	actorID string,
) (workspacepkg.CoordinationSetting, error) {
	current, err := loadCoordinationSetting(ctx, exec, ref)
	if err != nil {
		return workspacepkg.CoordinationSetting{}, err
	}
	if current.Revision != expectedRevision {
		return workspacepkg.CoordinationSetting{}, coordinationConflict(expectedRevision, current.Revision)
	}
	updatedAt := nextCoordinationCommandTimestamp(g.now().UTC(), current.UpdatedAt)
	query, args := coordinationCASQuery(ref, enabled, expectedRevision, updatedAt, actorID)
	result, err := exec.ExecContext(ctx, query, args...)
	if err != nil {
		return workspacepkg.CoordinationSetting{}, fmt.Errorf("store: set scoped network coordination: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return workspacepkg.CoordinationSetting{}, fmt.Errorf("store: read coordination CAS result: %w", err)
	}
	if affected != 1 {
		winner, loadErr := loadCoordinationSetting(ctx, exec, ref)
		if loadErr != nil {
			return workspacepkg.CoordinationSetting{}, loadErr
		}
		return workspacepkg.CoordinationSetting{}, coordinationConflict(expectedRevision, winner.Revision)
	}
	return loadCoordinationSetting(ctx, exec, ref)
}

func coordinationCASQuery(
	ref workspacepkg.CoordinationRef,
	enabled bool,
	expectedRevision int64,
	updatedAt time.Time,
	actorID string,
) (string, []any) {
	enabledValue := coordinationBoolInt64(enabled)
	timestamp := store.FormatTimestamp(updatedAt)
	if ref.ScopeKind == workspacepkg.InvitationScopeWorkspace {
		if expectedRevision == 0 {
			return `INSERT INTO workspace_network_coordination
                (workspace_id, enabled, revision, updated_at, updated_by)
                VALUES (?, ?, 1, ?, ?) ON CONFLICT(workspace_id) DO NOTHING`,
				[]any{ref.WorkspaceID, enabledValue, timestamp, strings.TrimSpace(actorID)}
		}
		return `UPDATE workspace_network_coordination
            SET enabled = ?, revision = revision + 1, updated_at = ?, updated_by = ?
            WHERE workspace_id = ? AND revision = ?`,
			[]any{enabledValue, timestamp, strings.TrimSpace(actorID), ref.WorkspaceID, expectedRevision}
	}
	if expectedRevision == 0 {
		return `INSERT INTO task_network_coordination
            (task_id, workspace_id, enabled, revision, updated_at, updated_by)
            VALUES (?, ?, ?, 1, ?, ?) ON CONFLICT(task_id) DO NOTHING`,
			[]any{ref.TaskID, ref.WorkspaceID, enabledValue, timestamp, strings.TrimSpace(actorID)}
	}
	return `UPDATE task_network_coordination
        SET enabled = ?, revision = revision + 1, updated_at = ?, updated_by = ?
        WHERE task_id = ? AND workspace_id = ? AND revision = ?`,
		[]any{enabledValue, timestamp, strings.TrimSpace(actorID), ref.TaskID, ref.WorkspaceID, expectedRevision}
}

func loadCoordinationSetting(
	ctx context.Context,
	exec taskSQLExecutor,
	ref workspacepkg.CoordinationRef,
) (workspacepkg.CoordinationSetting, error) {
	query := `SELECT enabled, revision, updated_at, updated_by
        FROM workspace_network_coordination WHERE workspace_id = ?`
	args := []any{ref.WorkspaceID}
	scopeID := ref.WorkspaceID
	if ref.ScopeKind == workspacepkg.InvitationScopeTask {
		query = `SELECT enabled, revision, updated_at, updated_by
            FROM task_network_coordination WHERE task_id = ? AND workspace_id = ?`
		args = []any{ref.TaskID, ref.WorkspaceID}
		scopeID = ref.TaskID
	}
	var enabled int64
	var revision int64
	var updatedRaw string
	var updatedBy string
	err := exec.QueryRowContext(ctx, query, args...).Scan(&enabled, &revision, &updatedRaw, &updatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return workspacepkg.CoordinationSetting{
			WorkspaceID: ref.WorkspaceID,
			ScopeKind:   ref.ScopeKind,
			ScopeID:     scopeID,
		}, nil
	}
	if err != nil {
		return workspacepkg.CoordinationSetting{}, fmt.Errorf("store: load scoped network coordination: %w", err)
	}
	if enabled != 0 && enabled != 1 {
		return workspacepkg.CoordinationSetting{}, fmt.Errorf("store: coordination enabled must be 0 or 1: %d", enabled)
	}
	updatedAt, err := store.ParseTimestamp(updatedRaw)
	if err != nil {
		return workspacepkg.CoordinationSetting{}, fmt.Errorf("store: parse coordination updated_at: %w", err)
	}
	return workspacepkg.CoordinationSetting{
		WorkspaceID: ref.WorkspaceID,
		ScopeKind:   ref.ScopeKind,
		ScopeID:     scopeID,
		Enabled:     enabled == 1,
		Revision:    revision,
		UpdatedAt:   updatedAt,
		UpdatedBy:   strings.TrimSpace(updatedBy),
	}, nil
}

func coordinationScopeID(ref workspacepkg.CoordinationRef) string {
	if ref.ScopeKind == workspacepkg.InvitationScopeTask {
		return ref.TaskID
	}
	return ref.WorkspaceID
}

func coordinationConflict(expected int64, actual int64) error {
	return fmt.Errorf(
		"%w: expected revision %d, current revision %d",
		workspacepkg.ErrCoordinationConflict,
		expected,
		actual,
	)
}

func nextCoordinationCommandTimestamp(now time.Time, previous time.Time) time.Time {
	normalized := now.UTC()
	if !previous.IsZero() && !normalized.After(previous) {
		return previous.Add(time.Nanosecond)
	}
	return normalized
}

func coordinationBoolInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
