package globaldb

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

const taskCatalogCTEBody = `,
base_runs AS MATERIALIZED (
	SELECT
		tr.id, tr.task_id, tr.workspace_id, tr.status, tr.attempt, tr.recovery_count,
		tr.previous_run_id, tr.failure_kind,
		tr.claimed_by_kind, tr.claimed_by_ref, tr.session_id, tr.lease_until,
		tr.heartbeat_at, tr.network_spec_json, tr.network_mode, tr.network_channel,
		tr.network_source, tr.queued_at, tr.claimed_at, tr.started_at,
		tr.ended_at, tr.error
	FROM base_tasks bt
	CROSS JOIN task_runs AS tr INDEXED BY idx_task_runs_task
	WHERE tr.task_id = bt.id
),
base_events AS MATERIALIZED (
	SELECT te.task_id, te.timestamp, te.event_seq
	FROM base_tasks bt
	CROSS JOIN task_events AS te INDEXED BY idx_task_events_task
	WHERE te.task_id = bt.id
),
base_dependency_edges AS MATERIALIZED (
	SELECT td.task_id, td.depends_on_task_id
	FROM base_tasks bt
	CROSS JOIN task_dependencies AS td INDEXED BY idx_task_dependencies_task
	WHERE td.task_id = bt.id
),
dependency_tasks AS MATERIALIZED (
	SELECT dependency.id, dependency.status, dependency.max_attempts
	FROM tasks dependency
	JOIN (
		SELECT DISTINCT depends_on_task_id
		FROM base_dependency_edges
	) edge ON edge.depends_on_task_id = dependency.id
),
dependency_runs AS MATERIALIZED (
	SELECT run.id, run.task_id, run.status, run.attempt, run.queued_at
	FROM dependency_tasks dependency
	CROSS JOIN task_runs AS run INDEXED BY idx_task_runs_task
	WHERE run.task_id = dependency.id
),
dependency_run_policy AS (
	SELECT
		task_id,
		MAX(CASE WHEN status IN ('starting', 'running') THEN 1 ELSE 0 END) AS has_active,
		MAX(CASE WHEN status IN ('queued', 'claimed') THEN 1 ELSE 0 END) AS has_queued_or_claimed,
		MAX(attempt) AS max_attempt
	FROM dependency_runs
	GROUP BY task_id
),
dependency_latest_terminal_candidates AS (
	SELECT
		task_id,
		status,
		ROW_NUMBER() OVER (
			PARTITION BY task_id
			ORDER BY attempt DESC, queued_at DESC, id DESC
		) AS row_number
	FROM dependency_runs
	WHERE status IN ('completed', 'failed', 'canceled')
),
dependency_completion AS (
	-- A dependency blocks only when its canonical status is not completed. Under
	-- taskStatusFromPolicySnapshot, completed depends on raw terminal state and
	-- run precedence; dependency blockers can never promote a task to completed.
	SELECT
		dependency.id AS task_id,
		CASE
			WHEN dependency.status IN ('canceled', 'draft') THEN 0
			WHEN COALESCE(policy.has_active, 0) = 1 THEN 0
			WHEN COALESCE(policy.has_queued_or_claimed, 0) = 1 THEN 0
			WHEN terminal.status = 'completed' THEN 1
			WHEN terminal.status = 'canceled' THEN 0
			WHEN terminal.status = 'failed'
				AND COALESCE(policy.max_attempt, 0) + 1 > dependency.max_attempts THEN 0
			WHEN dependency.status = 'completed' THEN 1
			ELSE 0
		END AS is_completed
	FROM dependency_tasks dependency
	LEFT JOIN dependency_run_policy policy ON policy.task_id = dependency.id
	LEFT JOIN dependency_latest_terminal_candidates terminal
		ON terminal.task_id = dependency.id AND terminal.row_number = 1
),
base_dependencies AS MATERIALIZED (
	SELECT edge.task_id, edge.depends_on_task_id, completion.is_completed AS dependency_completed
	FROM base_dependency_edges edge
	JOIN dependency_completion completion ON completion.task_id = edge.depends_on_task_id
),
run_activity AS (
	SELECT task_id, MAX(activity_at) AS last_activity_at
	FROM (
		SELECT task_id, queued_at AS activity_at FROM base_runs
		UNION ALL SELECT task_id, claimed_at FROM base_runs WHERE claimed_at IS NOT NULL
		UNION ALL SELECT task_id, started_at FROM base_runs WHERE started_at IS NOT NULL
		UNION ALL SELECT task_id, ended_at FROM base_runs WHERE ended_at IS NOT NULL
	)
	GROUP BY task_id
),
event_activity AS (
	SELECT task_id, MAX(timestamp) AS last_activity_at, MAX(event_seq) AS latest_event_seq
	FROM base_events
	GROUP BY task_id
),
child_counts AS (
	SELECT bt.id AS task_id, COUNT(child.id) AS child_count
	FROM base_tasks bt
	LEFT JOIN tasks AS child INDEXED BY idx_tasks_parent ON child.parent_task_id = bt.id
	GROUP BY bt.id
),
dependency_counts AS (
	SELECT task_id, COUNT(*) AS dependency_count
	FROM base_dependencies
	GROUP BY task_id
),
open_block_counts AS (
	SELECT bt.id AS task_id, COUNT(block.id) AS open_block_count
	FROM base_tasks bt
	LEFT JOIN task_blocks AS block INDEXED BY idx_task_blocks_open
		ON block.task_id = bt.id AND block.cleared_at IS NULL
	GROUP BY bt.id
),
run_policy AS (
	SELECT
		task_id,
		MAX(CASE WHEN status IN ('starting', 'running') THEN 1 ELSE 0 END) AS has_active,
		MAX(CASE WHEN status IN ('queued', 'claimed') THEN 1 ELSE 0 END) AS has_queued_or_claimed,
		MAX(attempt) AS max_attempt
	FROM base_runs
	GROUP BY task_id
),
latest_terminal_candidates AS (
	SELECT
		task_id,
		status,
		ROW_NUMBER() OVER (
			PARTITION BY task_id
			ORDER BY attempt DESC, queued_at DESC, id DESC
		) AS row_number
	FROM base_runs
	WHERE status IN ('completed', 'failed', 'canceled')
),
active_run_candidates AS (
	SELECT
		id, task_id, workspace_id, status, attempt, recovery_count, previous_run_id, failure_kind, claimed_by_kind,
		claimed_by_ref, session_id, lease_until, heartbeat_at,
		network_spec_json, network_mode, network_channel, network_source,
		queued_at, claimed_at, started_at,
		ended_at, error,
		ROW_NUMBER() OVER (
			PARTITION BY task_id
			ORDER BY
				CASE status
					WHEN 'running' THEN 4
					WHEN 'starting' THEN 3
					WHEN 'claimed' THEN 2
					WHEN 'queued' THEN 1
					ELSE 0
				END DESC,
				MAX(
					queued_at,
					COALESCE(claimed_at, queued_at),
					COALESCE(started_at, queued_at),
					COALESCE(ended_at, queued_at)
				) DESC,
				id DESC
		) AS row_number
	FROM base_runs
	WHERE status IN ('queued', 'claimed', 'starting', 'running')
),
catalog_derived AS (
	SELECT
		t.*,
		COALESCE(ea.latest_event_seq, 0) AS latest_event_seq,
		COALESCE(cc.child_count, 0) AS child_count,
		COALESCE(dc.dependency_count, 0) AS dependency_count,
		MAX(
			t.updated_at,
			COALESCE(ra.last_activity_at, t.updated_at),
			COALESCE(ea.last_activity_at, t.updated_at)
		) AS last_activity_at,
		CASE t.priority
			WHEN 'urgent' THEN 4
			WHEN 'high' THEN 3
			WHEN 'medium' THEN 2
			WHEN 'low' THEN 1
			ELSE 0
		END AS priority_rank,
		CASE
			WHEN t.status IN ('canceled', 'draft') THEN t.status
			WHEN COALESCE(rp.has_active, 0) = 1 THEN 'in_progress'
			WHEN lt.status = 'completed' AND COALESCE(rp.has_queued_or_claimed, 0) = 0 THEN 'completed'
			WHEN lt.status = 'failed'
				AND COALESCE(rp.has_queued_or_claimed, 0) = 0
				AND COALESCE(rp.max_attempt, 0) + 1 > t.max_attempts THEN 'failed'
			WHEN lt.status = 'canceled' AND COALESCE(rp.has_queued_or_claimed, 0) = 0 THEN 'canceled'
			WHEN t.status IN ('completed', 'failed', 'canceled')
				AND COALESCE(rp.has_queued_or_claimed, 0) = 0 THEN t.status
			WHEN t.needs_attention_at IS NOT NULL THEN 'needs_attention'
			WHEN COALESCE(rp.has_queued_or_claimed, 0) = 1 THEN
				CASE WHEN (
					t.paused = 1 OR
					(t.approval_policy = 'manual' AND t.approval_state <> 'approved') OR
					COALESCE(ob.open_block_count, 0) > 0 OR
					EXISTS (
						SELECT 1 FROM base_dependencies blocked_dependency
						WHERE blocked_dependency.task_id = t.id
							AND blocked_dependency.dependency_completed = 0
					)
				) THEN 'blocked' ELSE 'ready' END
			WHEN (
				t.paused = 1 OR
				(t.approval_policy = 'manual' AND t.approval_state <> 'approved') OR
				COALESCE(ob.open_block_count, 0) > 0 OR
				EXISTS (
					SELECT 1 FROM base_dependencies blocked_dependency
					WHERE blocked_dependency.task_id = t.id
						AND blocked_dependency.dependency_completed = 0
				)
			) THEN 'blocked'
			ELSE 'ready'
		END AS canonical_status,
		ar.id AS active_run_id,
		ar.workspace_id AS active_run_workspace_id,
		ar.network_channel AS network_channel,
		ar.status AS active_run_status,
		ar.attempt AS active_run_attempt,
		ar.recovery_count AS active_run_recovery_count,
		ar.previous_run_id AS active_run_previous_run_id,
		ar.failure_kind AS active_run_failure_kind,
		ar.claimed_by_kind AS active_run_claimed_by_kind,
		ar.claimed_by_ref AS active_run_claimed_by_ref,
		ar.session_id AS active_run_session_id,
		ar.lease_until AS active_run_lease_until,
		ar.heartbeat_at AS active_run_heartbeat_at,
		ar.network_spec_json AS active_run_network_spec_json,
		ar.network_mode AS active_run_network_mode,
		ar.network_channel AS active_run_network_channel,
		ar.network_source AS active_run_network_source,
		ar.queued_at AS active_run_queued_at,
		ar.claimed_at AS active_run_claimed_at,
		ar.started_at AS active_run_started_at,
		ar.ended_at AS active_run_ended_at,
		ar.error AS active_run_error
	FROM base_tasks t
	LEFT JOIN run_activity ra ON ra.task_id = t.id
	LEFT JOIN event_activity ea ON ea.task_id = t.id
	LEFT JOIN child_counts cc ON cc.task_id = t.id
	LEFT JOIN dependency_counts dc ON dc.task_id = t.id
	LEFT JOIN open_block_counts ob ON ob.task_id = t.id
	LEFT JOIN run_policy rp ON rp.task_id = t.id
	LEFT JOIN latest_terminal_candidates lt ON lt.task_id = t.id AND lt.row_number = 1
	LEFT JOIN active_run_candidates ar ON ar.task_id = t.id AND ar.row_number = 1
),
catalog AS (
	SELECT
		id, identifier, scope, workspace_id, parent_task_id, network_channel, title,
		priority, max_attempts, auto_enqueue_on_ready, canonical_status AS status,
		approval_policy, approval_state, owner_kind, owner_ref, current_run_id,
		latest_event_seq, created_by_kind, created_by_ref, origin_kind, origin_ref,
		created_at, updated_at, closed_at, needs_attention_reason, needs_attention_at,
		needs_attention_by_kind, needs_attention_by_ref, wake_creator, child_count,
		dependency_count, last_activity_at, priority_rank, active_run_id, active_run_workspace_id,
		active_run_status, active_run_attempt, active_run_recovery_count, active_run_previous_run_id,
		active_run_failure_kind, active_run_claimed_by_kind, active_run_claimed_by_ref,
		active_run_session_id, active_run_lease_until, active_run_heartbeat_at,
		active_run_network_spec_json, active_run_network_mode,
		active_run_network_channel, active_run_network_source,
		active_run_queued_at, active_run_claimed_at,
		active_run_started_at, active_run_ended_at, active_run_error
	FROM catalog_derived
)`

const taskCatalogSelectColumns = `id, identifier, scope, workspace_id, parent_task_id,
	title, priority, max_attempts, auto_enqueue_on_ready, status,
	approval_policy, approval_state, owner_kind, owner_ref, current_run_id,
	latest_event_seq, created_by_kind, created_by_ref, origin_kind, origin_ref,
	created_at, updated_at, closed_at, needs_attention_reason, needs_attention_at,
	needs_attention_by_kind, needs_attention_by_ref, wake_creator, child_count,
	dependency_count, last_activity_at, priority_rank, active_run_id, active_run_workspace_id, active_run_status,
	active_run_attempt, active_run_recovery_count, active_run_previous_run_id, active_run_failure_kind,
	active_run_claimed_by_kind, active_run_claimed_by_ref, active_run_session_id,
	active_run_lease_until, active_run_heartbeat_at, active_run_network_spec_json,
	active_run_network_mode, active_run_network_channel, active_run_network_source,
	active_run_queued_at, active_run_claimed_at, active_run_started_at,
	active_run_ended_at, active_run_error`

const taskCatalogBaseColumns = `id, identifier, scope, workspace_id, parent_task_id,
	title, priority, max_attempts, auto_enqueue_on_ready, status,
	approval_policy, approval_state, owner_kind, owner_ref, current_run_id,
	created_by_kind, created_by_ref, origin_kind, origin_ref, created_at, updated_at,
	closed_at, paused, needs_attention_reason, needs_attention_at,
	needs_attention_by_kind, needs_attention_by_ref, wake_creator`

func taskCatalogBaseFilter(query taskpkg.CatalogQuery) ([]string, []any) {
	where, args := store.BuildClauses(
		store.StringClause("priority", string(query.Priority)),
		store.StringClause("approval_state", string(query.ApprovalState)),
		store.StringClause("owner_kind", string(query.OwnerKind)),
		store.StringClause("owner_ref", query.OwnerRef),
		store.StringClause("parent_task_id", query.ParentTaskID),
	)
	switch query.Scope {
	case taskpkg.CatalogScopeGlobal:
		where = append(where, "scope = ?")
		args = append(args, string(taskpkg.ScopeGlobal))
	case taskpkg.CatalogScopeWorkspace:
		where = append(where, "scope = ?", "workspace_id = ?")
		args = append(args, string(taskpkg.ScopeWorkspace), query.WorkspaceID)
	case taskpkg.CatalogScopeAll:
		if query.WorkspaceID != "" {
			where = append(where, "(scope = ? OR (scope = ? AND workspace_id = ?))")
			args = append(args, string(taskpkg.ScopeGlobal), string(taskpkg.ScopeWorkspace), query.WorkspaceID)
		}
	}
	if query.Status == "" && !query.IncludeDrafts {
		where = append(where, "status <> ?")
		args = append(args, string(taskpkg.TaskStatusDraft))
	}
	if query.Search != "" {
		needle := strings.ToLower(query.Search)
		where = append(where, "(instr(LOWER(title), ?) > 0 OR instr(LOWER(COALESCE(identifier, '')), ?) > 0)")
		args = append(args, needle, needle)
	}
	return where, args
}

func taskCatalogFilter(query taskpkg.CatalogQuery) ([]string, []any) {
	return store.BuildClauses(
		store.StringClause("status", string(query.Status)),
		store.StringClause("network_channel", query.ParticipationChannel),
	)
}

func taskCatalogPageFilter(
	query taskpkg.CatalogQuery,
	where []string,
	args []any,
) ([]string, []any, error) {
	if query.Cursor == "" {
		return where, args, nil
	}
	cursor, err := taskpkg.DecodeCatalogCursor(query)
	if err != nil {
		return nil, nil, err
	}
	activity := store.FormatTimestamp(cursor.ActivityAt)
	if query.Sort == taskpkg.CatalogSortPriority {
		where = append(where, `(priority_rank < ? OR (
			priority_rank = ? AND (last_activity_at < ? OR (
				last_activity_at = ? AND id < ?
			))
		))`)
		args = append(args, cursor.PriorityRank, cursor.PriorityRank, activity, activity, cursor.TaskID)
		return where, args, nil
	}
	where = append(where, "(last_activity_at < ? OR (last_activity_at = ? AND id < ?))")
	args = append(args, activity, activity, cursor.TaskID)
	return where, args, nil
}

func taskCatalogOrderBy(sortKey taskpkg.CatalogSort) string {
	if sortKey == taskpkg.CatalogSortPriority {
		return " ORDER BY priority_rank DESC, last_activity_at DESC, id DESC"
	}
	return " ORDER BY last_activity_at DESC, id DESC"
}

func taskCatalogStatement(selectSQL string, baseWhere []string, where []string) string {
	base := store.AppendWhere("SELECT "+taskCatalogBaseColumns+" FROM tasks", baseWhere)
	return "WITH base_tasks AS MATERIALIZED (" + base + ")" + taskCatalogCTEBody + " " +
		store.AppendWhere(selectSQL, where)
}

func taskCatalogLimitClause(limit int) string {
	return fmt.Sprintf(" LIMIT %d", limit)
}
