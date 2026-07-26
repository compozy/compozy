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

var _ taskpkg.RecordStore = (*TaskRepo)(nil)
var _ taskpkg.RunStore = (*GlobalDB)(nil)
var _ taskpkg.AutonomyLeaseStore = (*TaskRunRepo)(nil)
var _ taskpkg.DeleteTaskTransactionStore = (*TaskRepo)(nil)

const taskListOrderByActivitySQL = ` ORDER BY COALESCE((
	SELECT MAX(activity_at)
	FROM (
		SELECT tasks.updated_at AS activity_at
		UNION ALL
		SELECT tasks.created_at AS activity_at
		UNION ALL
		SELECT tr.queued_at AS activity_at
		FROM task_runs tr
		WHERE tr.task_id = tasks.id
		UNION ALL
		SELECT tr.claimed_at AS activity_at
		FROM task_runs tr
		WHERE tr.task_id = tasks.id AND tr.claimed_at IS NOT NULL
		UNION ALL
		SELECT tr.started_at AS activity_at
		FROM task_runs tr
		WHERE tr.task_id = tasks.id AND tr.started_at IS NOT NULL
		UNION ALL
		SELECT tr.ended_at AS activity_at
		FROM task_runs tr
		WHERE tr.task_id = tasks.id AND tr.ended_at IS NOT NULL
		UNION ALL
		SELECT te.timestamp AS activity_at
		FROM task_events te
		WHERE te.task_id = tasks.id
	)
), tasks.updated_at) DESC, updated_at DESC, created_at DESC, id DESC`

const taskRunSelectColumnsSQL = `
	id, task_id, workspace_id, run_kind, loop_run_id, status, attempt, recovery_count, previous_run_id, failure_kind,
	claimed_by_kind, claimed_by_ref, session_id, origin_kind, origin_ref, idempotency_key,
	network_spec_json, network_mode, network_channel, network_source,
	designation_group_id, '' AS claim_token,
	claim_token_hash, lease_until, heartbeat_at, queued_at,
	claimed_at, started_at, ended_at, tokens_used, error, metadata_json, result_json, review_required,
	review_request_round, review_policy_snapshot, review_request_id, parent_run_id, review_id,
	review_round, continuation_reason, missing_work_json, next_round_guidance,
	network_wake_id, network_target_session_id, network_owner_key`

const taskLatestEventSeqSelectSQL = `COALESCE((
	SELECT MAX(te.event_seq)
	FROM task_events te
	WHERE te.task_id = tasks.id
), 0)`

// CreateTask inserts one durable task record.
func (g *TaskRepo) CreateTask(ctx context.Context, record taskpkg.Task) error {
	if err := g.checkReady(ctx, "create task"); err != nil {
		return err
	}

	normalized, err := g.normalizeTaskForCreate(record)
	if err != nil {
		return err
	}
	if err := g.ensureTaskCreateReferences(ctx, normalized); err != nil {
		return err
	}

	if err := insertTaskWithExecutor(ctx, g.db, normalized); err != nil {
		return fmt.Errorf("store: create task %q: %w", normalized.ID, err)
	}

	return nil
}

// DeleteTask removes one durable task record and any ON DELETE CASCADE children
// owned by the task tables.
func (g *TaskRepo) DeleteTask(ctx context.Context, id string) error {
	if err := g.checkReady(ctx, "delete task"); err != nil {
		return err
	}

	return g.deleteTaskWithExecutor(ctx, g.db, id)
}

// WithDeleteTaskTransaction executes one delete-task mutation flow inside a
// single immediate transaction so reconciliation failures can roll back the
// primary delete.
func (g *TaskRepo) WithDeleteTaskTransaction(
	ctx context.Context,
	fn func(taskpkg.DeleteTaskMutationStore) error,
) error {
	if err := g.checkReady(ctx, "delete task transaction"); err != nil {
		return err
	}

	return g.withTaskImmediateTransaction(ctx, "delete task", func(exec taskSQLExecutor) error {
		return fn(&deleteTaskTxStore{tasks: g, exec: exec})
	})
}

func (g *TaskRepo) deleteTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	id string,
) error {
	trimmedID, err := requireTaskValue(id, "task id")
	if err != nil {
		return err
	}

	affected, err := sqlcgen.New(exec).DeleteTask(ctx, trimmedID)
	if err != nil {
		return mapTaskDeleteConstraintError(trimmedID, err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: task %q", taskpkg.ErrTaskNotFound, trimmedID)
	}
	return nil
}

func mapTaskDeleteConstraintError(id string, err error) error {
	if err == nil {
		return nil
	}

	if isSQLiteForeignKeyConstraint(err) {
		return fmt.Errorf(
			"%w: task %q has child tasks; delete children first",
			taskpkg.ErrValidation,
			id,
		)
	}
	return fmt.Errorf("store: delete task %q: %w", id, err)
}

// UpdateTask replaces the persisted canonical task record.
func (g *TaskRepo) UpdateTask(ctx context.Context, record taskpkg.Task, actor taskpkg.ActorContext) error {
	if err := g.checkReady(ctx, "update task"); err != nil {
		return err
	}

	return g.withTaskImmediateTransaction(ctx, "update task", func(exec taskSQLExecutor) error {
		return g.updateTaskWithExecutor(ctx, exec, record, actor)
	})
}

func (g *TaskRepo) updateTaskWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	record taskpkg.Task,
	actor taskpkg.ActorContext,
) error {
	normalized, err := g.normalizeTaskForUpdate(record)
	if err != nil {
		return err
	}

	current, err := g.getTaskWithExecutor(ctx, exec, normalized.ID)
	if err != nil {
		return err
	}
	if err := taskpkg.ValidateImmutableTaskFields(current, normalized); err != nil {
		return err
	}

	normalized.CreatedAt = current.CreatedAt
	statusChanged, err := taskStatusChangedForUpdate(current.Status, normalized.Status, actor)
	if err != nil {
		return err
	}

	affected, err := sqlcgen.New(exec).UpdateTask(ctx, updateTaskParams(normalized))
	if err != nil {
		return fmt.Errorf("store: update task %q: %w", normalized.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: task %q", taskpkg.ErrTaskNotFound, normalized.ID)
	}
	return setTaskStatusIfChangedWithExecutor(ctx, exec, current, normalized, actor, statusChanged)
}

// GetTask returns one persisted task by primary key.
func (g *TaskRepo) GetTask(ctx context.Context, id string) (taskpkg.Task, error) {
	if err := g.checkReady(ctx, "get task"); err != nil {
		return taskpkg.Task{}, err
	}

	trimmedID, err := requireTaskValue(id, "task id")
	if err != nil {
		return taskpkg.Task{}, err
	}

	row, err := g.queries.GetTask(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.Task{}, taskpkg.ErrTaskNotFound
		}
		return taskpkg.Task{}, err
	}
	return taskFromGenerated(&row)
}

// ListTasks returns durable task summaries that match the supplied filters.
func (g *TaskRepo) ListTasks(
	ctx context.Context,
	query taskpkg.Query,
) (summaries []taskpkg.Summary, err error) {
	if err := g.checkReady(ctx, "list tasks"); err != nil {
		return nil, err
	}
	if err := query.Validate("task_query"); err != nil {
		return nil, err
	}

	normalized := normalizeTaskQuery(query)
	// dynamic-sql: optional task filters, full-text matching, activity ordering, and limit alter the query shape.
	sqlQuery := `SELECT
		id, identifier, scope, workspace_id, parent_task_id, title, description,
		priority, max_attempts, auto_enqueue_on_ready, status, approval_policy, approval_state,
		owner_kind, owner_ref, created_by_kind, created_by_ref, origin_kind, origin_ref,
		created_at, updated_at, closed_at, current_run_id, ` + taskLatestEventSeqSelectSQL + `,
		paused, paused_by, paused_at, paused_reason, needs_attention_reason,
		needs_attention_at, needs_attention_by_kind, needs_attention_by_ref, wake_creator,
		metadata_json
		FROM tasks`
	where, args := store.BuildClauses(
		store.StringClause("scope", string(normalized.Scope)),
		store.StringClause("workspace_id", normalized.WorkspaceID),
		store.StringClause("status", string(normalized.Status)),
		store.StringClause("priority", string(normalized.Priority)),
		store.StringClause("approval_state", string(normalized.ApprovalState)),
		store.StringClause("owner_kind", string(normalized.OwnerKind)),
		store.StringClause("owner_ref", normalized.OwnerRef),
		store.StringClause("parent_task_id", normalized.ParentTaskID),
		store.StringClause("created_by_kind", string(normalized.CreatedByKind)),
		store.StringClause("created_by_ref", normalized.CreatedByRef),
	)
	where, args = appendTaskSearchClause(where, args, normalized.Search)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += taskListOrderByActivitySQL
	sqlQuery, args = store.AppendLimit(sqlQuery, args, normalized.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query tasks: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "task query")
	}()

	summaries = make([]taskpkg.Summary, 0)
	for rows.Next() {
		record, scanErr := scanTaskRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		summaries = append(summaries, taskSummaryFromRecord(record))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate tasks: %w", err)
	}

	return summaries, nil
}

func appendTaskSearchClause(where []string, args []any, search string) ([]string, []any) {
	trimmedSearch := strings.TrimSpace(search)
	if trimmedSearch == "" {
		return where, args
	}

	likePattern := "%" + strings.ToLower(trimmedSearch) + "%"
	where = append(where, "(LOWER(title) LIKE ? OR LOWER(COALESCE(identifier, '')) LIKE ?)")
	args = append(args, likePattern, likePattern)
	return where, args
}

// CountDirectChildren reports how many persisted tasks reference the supplied parent id.
func (g *TaskRepo) CountDirectChildren(ctx context.Context, parentTaskID string) (int, error) {
	if err := g.checkReady(ctx, "count task children"); err != nil {
		return 0, err
	}

	return g.countDirectChildrenWithExecutor(ctx, g.db, parentTaskID)
}

func (g *TaskRepo) countDirectChildrenWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	parentTaskID string,
) (int, error) {
	trimmedID, err := requireTaskValue(parentTaskID, "parent task id")
	if err != nil {
		return 0, err
	}

	count, err := sqlcgen.New(exec).CountDirectTaskChildren(ctx, nullableTaskString(trimmedID))
	if err != nil {
		return 0, fmt.Errorf("store: count direct children for task %q: %w", trimmedID, err)
	}
	return int(count), nil
}

// CreateTaskRun inserts one durable task-run record.
