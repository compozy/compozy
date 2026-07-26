package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

// CountPausedTasks returns the number of directly paused tasks.
func (g *TaskRepo) CountPausedTasks(ctx context.Context) (int, error) {
	if err := g.checkReady(ctx, "count paused tasks"); err != nil {
		return 0, err
	}
	count, err := g.queries.CountPausedTasks(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: count paused tasks: %w", err)
	}
	return int(count), nil
}

// CountActiveTaskRunClaims returns currently leased task runs.
func (g *TaskRepo) CountActiveTaskRunClaims(ctx context.Context) (int, error) {
	if err := g.checkReady(ctx, "count active task-run claims"); err != nil {
		return 0, err
	}
	count, err := g.queries.CountActiveTaskRunClaims(ctx, sqlcgen.CountActiveTaskRunClaimsParams{
		ClaimedStatus:  taskpkg.TaskRunStatusClaimed.String(),
		StartingStatus: taskpkg.TaskRunStatusStarting.String(),
		RunningStatus:  taskpkg.TaskRunStatusRunning.String(),
	})
	if err != nil {
		return 0, fmt.Errorf("store: count active task-run claims: %w", err)
	}
	return int(count), nil
}

// CountQueuedTaskRuns returns queued run pressure, optionally excluding paused tasks.
func (g *TaskRepo) CountQueuedTaskRuns(ctx context.Context, includePaused bool) (int, error) {
	if err := g.checkReady(ctx, "count queued task runs"); err != nil {
		return 0, err
	}
	if includePaused {
		count, err := g.queries.CountAllQueuedTaskRuns(ctx, taskpkg.TaskRunStatusQueued.String())
		if err != nil {
			return 0, fmt.Errorf("store: count queued task runs: %w", err)
		}
		return int(count), nil
	}
	// dynamic-sql: sqlc's SQLite analyzer cannot resolve the correlated recursive ancestor CTE.
	query := `SELECT COUNT(1)
		FROM task_runs tr
		JOIN tasks t ON t.id = tr.task_id
		WHERE tr.status = ?`
	args := []any{taskpkg.TaskRunStatusQueued.String()}
	query += " AND " + effectiveTaskPauseExclusionSQL()
	var count int
	if err := g.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: count queued task runs: %w", err)
	}
	return count, nil
}

// CountEscalatingQueuedTaskRuns counts active durable convergence episodes for claimable queued runs.
func (g *TaskRepo) CountEscalatingQueuedTaskRuns(ctx context.Context) (int, error) {
	if err := g.checkReady(ctx, "count escalating queued task runs"); err != nil {
		return 0, err
	}
	// dynamic-sql: sqlc's SQLite analyzer cannot resolve the correlated recursive ancestor CTE.
	query := `SELECT COUNT(1)
		FROM task_run_starvation trs
		JOIN task_runs tr ON tr.id = trs.run_id
		JOIN tasks t ON t.id = tr.task_id
		WHERE tr.status = ?
		  AND t.status NOT IN (?, ?, ?, ?)
		  AND ` + effectiveTaskPauseExclusionSQL()
	var count int
	if err := g.db.QueryRowContext(
		ctx,
		query,
		taskpkg.TaskRunStatusQueued.String(),
		string(taskpkg.TaskStatusDraft),
		string(taskpkg.TaskStatusBlocked),
		string(taskpkg.TaskStatusNeedsAttention),
		string(taskpkg.TaskStatusCanceled),
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("store: count escalating queued task runs: %w", err)
	}
	return count, nil
}

// CountNeedsAttentionTaskRuns counts runs the convergence backstop escalated to needs_attention.
func (g *TaskRepo) CountNeedsAttentionTaskRuns(ctx context.Context) (int, error) {
	if err := g.checkReady(ctx, "count needs attention task runs"); err != nil {
		return 0, err
	}
	count, err := g.queries.CountNeedsAttentionTaskRuns(ctx, taskpkg.TaskRunStatusNeedsAttention.String())
	if err != nil {
		return 0, fmt.Errorf("store: count needs attention task runs: %w", err)
	}
	return int(count), nil
}

// SchedulerBacklog returns queued task runs ordered by scheduler priority.
func (g *TaskRepo) SchedulerBacklog(
	ctx context.Context,
	query taskpkg.SchedulerBacklogQuery,
) (backlog taskpkg.SchedulerBacklog, err error) {
	if err := g.checkReady(ctx, "list scheduler backlog"); err != nil {
		return taskpkg.SchedulerBacklog{}, err
	}
	normalized := normalizeSchedulerBacklogQuery(query)
	if err := normalized.Validate("scheduler_backlog"); err != nil {
		return taskpkg.SchedulerBacklog{}, err
	}
	// dynamic-sql: optional scope/workspace/pause filters and ordering produce a structural scheduler query.
	whereSQL, args := schedulerBacklogWhere(normalized)

	var total int
	var countBuilder strings.Builder
	countBuilder.WriteString(`SELECT COUNT(1)
		FROM task_runs tr
		JOIN tasks t ON t.id = tr.task_id
		WHERE `)
	countBuilder.WriteString(whereSQL)
	countQuery := countBuilder.String()
	if err := g.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return taskpkg.SchedulerBacklog{}, fmt.Errorf("store: count scheduler backlog: %w", err)
	}

	var rowBuilder strings.Builder
	rowBuilder.WriteString(`SELECT tr.id
		FROM task_runs tr
		JOIN tasks t ON t.id = tr.task_id
		WHERE `)
	rowBuilder.WriteString(whereSQL)
	rowBuilder.WriteString(`
		ORDER BY `)
	rowBuilder.WriteString(taskPriorityValueSQL)
	rowBuilder.WriteString(` DESC, tr.queued_at ASC, tr.id ASC`)
	rowQuery := rowBuilder.String()
	rowQuery, args = store.AppendLimit(rowQuery, args, normalized.Limit)
	rows, err := g.db.QueryContext(ctx, rowQuery, args...)
	if err != nil {
		return taskpkg.SchedulerBacklog{}, fmt.Errorf("store: query scheduler backlog: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "scheduler backlog")
	}()

	items := make([]taskpkg.SchedulerBacklogRun, 0)
	for rows.Next() {
		var runID string
		if err := rows.Scan(&runID); err != nil {
			return taskpkg.SchedulerBacklog{}, fmt.Errorf("store: scan scheduler backlog run id: %w", err)
		}
		run, err := g.GetTaskRun(ctx, runID)
		if err != nil {
			return taskpkg.SchedulerBacklog{}, err
		}
		taskRecord, err := g.GetTask(ctx, run.TaskID)
		if err != nil {
			return taskpkg.SchedulerBacklog{}, err
		}
		effectivePaused, pausedByTaskID, err := g.IsTaskEffectivelyPaused(ctx, taskRecord.ID)
		if err != nil {
			return taskpkg.SchedulerBacklog{}, err
		}
		items = append(items, taskpkg.SchedulerBacklogRun{
			Task:            taskRecord,
			Run:             run,
			EffectivePaused: effectivePaused,
			PausedByTaskID:  pausedByTaskID,
		})
	}
	if err := rows.Err(); err != nil {
		return taskpkg.SchedulerBacklog{}, fmt.Errorf("store: iterate scheduler backlog: %w", err)
	}
	return taskpkg.SchedulerBacklog{Runs: items, Total: total}, nil
}

func schedulerBacklogWhere(query taskpkg.SchedulerBacklogQuery) (string, []any) {
	where := []string{"tr.status = ?"}
	args := []any{taskpkg.TaskRunStatusQueued.String()}
	if query.Scope.Normalize() != "" {
		where = append(where, "t.scope = ?")
		args = append(args, string(query.Scope))
	}
	if query.WorkspaceID != "" {
		where = append(where, "t.workspace_id = ?")
		args = append(args, query.WorkspaceID)
	}
	if !query.IncludePaused {
		where = append(where, effectiveTaskPauseExclusionSQL())
	}
	return strings.Join(where, " AND "), args
}

func normalizeSchedulerBacklogQuery(query taskpkg.SchedulerBacklogQuery) taskpkg.SchedulerBacklogQuery {
	normalized := query
	normalized.Scope = normalized.Scope.Normalize()
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	if normalized.Limit <= 0 {
		normalized.Limit = 50
	}
	if normalized.Limit > 500 {
		normalized.Limit = 500
	}
	return normalized
}
