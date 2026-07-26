package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (g *TaskRepo) CreateTaskRun(ctx context.Context, run taskpkg.Run) error {
	if err := g.checkReady(ctx, "create task run"); err != nil {
		return err
	}

	normalized, err := g.normalizeTaskRunForCreate(run)
	if err != nil {
		return err
	}

	return g.withTaskImmediateTransaction(ctx, "create task run", func(exec taskSQLExecutor) error {
		if normalized.IsTaskAnchored() {
			taskRecord, loadErr := g.getTaskWithExecutor(ctx, exec, normalized.TaskID)
			if loadErr != nil {
				return loadErr
			}
			normalized, loadErr = bindTaskRunWorkspace(normalized, taskRecord)
			if loadErr != nil {
				return loadErr
			}
		}
		if err := insertTaskRunWithExecutor(ctx, exec, normalized); err != nil {
			return err
		}
		return replaceTaskRunCapabilitiesWithExecutor(ctx, exec, normalized)
	})
}

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

// UpdateTaskRun replaces the persisted canonical task-run record.
func (g *TaskRepo) UpdateTaskRun(ctx context.Context, run taskpkg.Run) error {
	if err := g.checkReady(ctx, "update task run"); err != nil {
		return err
	}

	normalized, err := g.normalizeTaskRunForUpdate(run)
	if err != nil {
		return err
	}

	return g.withTaskImmediateTransaction(ctx, "update task run", func(exec taskSQLExecutor) error {
		return g.updateTaskRunWithExecutor(ctx, exec, normalized)
	})
}

// CompleteRunSettlement completes one unfenced run and atomically applies
// successful task-hierarchy aggregation.
func (g *TaskRepo) CompleteRunSettlement(
	ctx context.Context,
	run taskpkg.Run,
	actor taskpkg.ActorContext,
) (taskpkg.CompletedRunSettlement, error) {
	if err := g.checkReady(ctx, "complete task run settlement"); err != nil {
		return taskpkg.CompletedRunSettlement{}, err
	}
	if err := actor.Validate(); err != nil {
		return taskpkg.CompletedRunSettlement{}, err
	}
	normalized, err := g.normalizeTaskRunForUpdate(run)
	if err != nil {
		return taskpkg.CompletedRunSettlement{}, err
	}
	if normalized.Status.Normalize() != taskpkg.TaskRunStatusCompleted {
		return taskpkg.CompletedRunSettlement{}, fmt.Errorf(
			"%w: completed run settlement requires completed status",
			taskpkg.ErrInvalidStatusTransition,
		)
	}

	var settlement taskpkg.CompletedRunSettlement
	if err := g.withTaskImmediateTransaction(ctx, "complete task run settlement", func(exec taskSQLExecutor) error {
		if err := g.updateTaskRunWithExecutor(ctx, exec, normalized); err != nil {
			return err
		}
		settlement, err = g.settleCompletedTaskHierarchyWithExecutor(
			ctx,
			exec,
			normalized.TaskID,
			actor,
			normalized.EndedAt,
		)
		if err != nil {
			return err
		}
		settlement.Run = normalized
		return nil
	}); err != nil {
		return taskpkg.CompletedRunSettlement{}, err
	}
	return settlement, nil
}

func (g *TaskRepo) updateTaskRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	normalized taskpkg.Run,
) error {
	current, err := g.getTaskRunWithExecutor(ctx, exec, normalized.ID)
	if err != nil {
		return err
	}
	if err := taskpkg.ValidateImmutableRunFields(current, normalized); err != nil {
		return err
	}
	currentSessionID := strings.TrimSpace(current.SessionID)
	nextSessionID := strings.TrimSpace(normalized.SessionID)
	if currentSessionID != "" &&
		nextSessionID != currentSessionID &&
		(nextSessionID != "" || normalized.Status.Normalize() != taskpkg.TaskRunStatusQueued) &&
		!allowsManagedTaskRunStartSessionTransfer(current, normalized) {
		return taskpkg.ErrSessionAlreadyBound
	}
	if normalized.QueuedAt.IsZero() {
		normalized.QueuedAt = current.QueuedAt
	}
	if normalized.IsTaskAnchored() {
		if err := g.ensureTaskExistsWithExecutor(ctx, exec, normalized.TaskID); err != nil {
			return err
		}
	}
	if err := updateTaskRunRecordWithExecutor(ctx, exec, normalized); err != nil {
		return err
	}
	if normalized.IsTaskAnchored() {
		if err := updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, current, normalized); err != nil {
			return err
		}
	}
	return replaceTaskRunCapabilitiesWithExecutor(ctx, exec, normalized)
}

func updateTaskRunRecordWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
) error {
	params, err := updateTaskRunParams(run)
	if err != nil {
		return err
	}
	affected, err := sqlcgen.New(exec).UpdateTaskRun(ctx, params)
	if err != nil {
		return fmt.Errorf("store: update task run %q: %w", run.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: task run %q", taskpkg.ErrTaskRunNotFound, run.ID)
	}
	return nil
}

func allowsManagedTaskRunStartSessionTransfer(current taskpkg.Run, next taskpkg.Run) bool {
	currentStatus := current.Status.Normalize()
	nextStatus := next.Status.Normalize()
	currentSessionID := strings.TrimSpace(current.SessionID)
	if strings.TrimSpace(next.SessionID) == "" {
		return false
	}
	if current.ClaimedBy == nil ||
		current.ClaimedBy.Kind.Normalize() != taskpkg.ActorKindAgentSession ||
		strings.TrimSpace(current.ClaimedBy.Ref) != currentSessionID {
		return false
	}
	if currentStatus == taskpkg.TaskRunStatusClaimed &&
		nextStatus == taskpkg.TaskRunStatusStarting {
		return true
	}
	return currentStatus == taskpkg.TaskRunStatusStarting &&
		nextStatus == taskpkg.TaskRunStatusStarting &&
		current.StartedAt.IsZero() &&
		next.StartedAt.IsZero()
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
