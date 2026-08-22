package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func bindTaskRunWorkspace(run taskpkg.Run, taskRecord taskpkg.Task) (taskpkg.Run, error) {
	taskWorkspaceID := strings.TrimSpace(taskRecord.WorkspaceID)
	specWorkspaceID := strings.TrimSpace(run.NetworkSpecSnapshot().WorkspaceID)
	if taskWorkspaceID != "" && specWorkspaceID != "" && taskWorkspaceID != specWorkspaceID {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: task run workspace %q conflicts with task workspace %q",
			taskpkg.ErrInvalidScopeBinding,
			specWorkspaceID,
			taskWorkspaceID,
		)
	}
	authoritativeWorkspaceID := taskWorkspaceID
	if authoritativeWorkspaceID == "" {
		authoritativeWorkspaceID = specWorkspaceID
	}
	if run.WorkspaceID == "" {
		run.WorkspaceID = authoritativeWorkspaceID
	} else if authoritativeWorkspaceID != "" && run.WorkspaceID != authoritativeWorkspaceID {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: task run workspace %q conflicts with authoritative workspace %q",
			taskpkg.ErrInvalidScopeBinding,
			run.WorkspaceID,
			authoritativeWorkspaceID,
		)
	}
	if err := run.Validate(); err != nil {
		return taskpkg.Run{}, err
	}
	return run, nil
}

// GetTaskRun returns one persisted task run by primary key.
func (g *TaskRepo) GetTaskRun(ctx context.Context, id string) (taskpkg.Run, error) {
	if err := g.checkReady(ctx, "get task run"); err != nil {
		return taskpkg.Run{}, err
	}

	trimmedID, err := requireTaskValue(id, "task run id")
	if err != nil {
		return taskpkg.Run{}, err
	}

	row, err := g.queries.GetTaskRun(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
		}
		return taskpkg.Run{}, err
	}
	run, err := taskRunFromGenerated(&row)
	if err != nil {
		return taskpkg.Run{}, err
	}
	return g.loadTaskRunCapabilities(ctx, g.db, run)
}

// ListTaskRuns returns persisted runs that match the supplied filters.
func (g *TaskRepo) ListTaskRuns(
	ctx context.Context,
	query taskpkg.RunQuery,
) ([]taskpkg.Run, error) {
	if err := g.checkReady(ctx, "list task runs"); err != nil {
		return nil, err
	}

	return g.listTaskRunsWithExecutor(ctx, g.db, query)
}

func (g *TaskRepo) listTaskRunsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	query taskpkg.RunQuery,
) (runs []taskpkg.Run, err error) {
	if err := query.Validate("task_run_query"); err != nil {
		return nil, err
	}

	normalized := normalizeTaskRunQuery(query)
	// dynamic-sql: optional run filters and limit alter the query structure.
	sqlQuery := `SELECT ` + taskRunSelectColumnsSQL + ` FROM task_runs`
	where, args := store.BuildClauses(
		store.StringClause("task_id", normalized.TaskID),
		store.StringClause("status", normalized.Status.String()),
		store.StringClause("session_id", normalized.SessionID),
		store.StringClause("designation_group_id", normalized.DesignationGroupID),
		store.StringClause("network_channel", normalized.ParticipationChannel),
	)
	if normalized.ReadScope != (store.ReadScope{}) && !normalized.ReadScope.AllProfiles {
		profilePredicate := "EXISTS (SELECT 1 FROM tasks WHERE tasks.id = task_runs.task_id AND tasks.profile_id = ?)"
		where = append(where, profilePredicate)
		args = append(args, normalized.ReadScope.ProfileID)
	}
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY queued_at DESC, id DESC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, normalized.Limit)

	rows, err := exec.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query task runs: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "task run query")
	}()

	runs = make([]taskpkg.Run, 0)
	for rows.Next() {
		run, scanErr := scanTaskRunRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate task runs: %w", err)
	}

	return g.loadTaskRunCapabilitiesForList(ctx, exec, runs)
}

// ListTaskRunsByStatus returns persisted runs that match any of the supplied statuses.
func (g *TaskRepo) ListTaskRunsByStatus(
	ctx context.Context,
	statuses []taskpkg.RunStatus,
) ([]taskpkg.Run, error) {
	if err := g.checkReady(ctx, "list task runs by status"); err != nil {
		return nil, err
	}
	if len(statuses) == 0 {
		return []taskpkg.Run{}, nil
	}

	args := make([]string, 0, len(statuses))
	for _, status := range statuses {
		normalized := status.Normalize()
		if err := normalized.Validate("task_run_statuses"); err != nil {
			return nil, err
		}
		args = append(args, normalized.String())
	}

	rows, err := g.queries.ListTaskRunsByStatus(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("store: query task runs by status: %w", err)
	}

	runs := make([]taskpkg.Run, 0, len(rows))
	for index := range rows {
		run, mapErr := taskRunFromStatusGenerated(&rows[index])
		if mapErr != nil {
			return nil, mapErr
		}
		runs = append(runs, run)
	}

	return g.loadTaskRunCapabilitiesForList(ctx, g.db, runs)
}

// CountActiveSessionBindings reports how many non-terminal runs are bound to one session.
func (g *TaskRepo) CountActiveSessionBindings(ctx context.Context, sessionID string) (int, error) {
	if err := g.checkReady(ctx, "count active task-run session bindings"); err != nil {
		return 0, err
	}

	trimmedSessionID, err := requireTaskValue(sessionID, "task run session id")
	if err != nil {
		return 0, err
	}

	count, err := g.queries.CountActiveTaskRunSessionBindings(
		ctx,
		sqlcgen.CountActiveTaskRunSessionBindingsParams{
			SessionID:      nullableTaskString(trimmedSessionID),
			ClaimedStatus:  taskpkg.TaskRunStatusClaimed.String(),
			StartingStatus: taskpkg.TaskRunStatusStarting.String(),
			RunningStatus:  taskpkg.TaskRunStatusRunning.String(),
		},
	)
	if err != nil {
		return 0, fmt.Errorf(
			"store: count active task-run session bindings for %q: %w",
			trimmedSessionID,
			err,
		)
	}

	return int(count), nil
}
