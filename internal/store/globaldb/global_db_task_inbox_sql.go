package globaldb

import (
	"strings"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

const taskInboxCTESuffix = `,
latest_inbox_run_candidates AS (
	SELECT
		id, task_id, workspace_id, status, attempt, recovery_count, previous_run_id, failure_kind, claimed_by_kind,
		claimed_by_ref, session_id, lease_until, heartbeat_at,
		network_spec_json, network_mode, network_channel, network_source,
		queued_at, claimed_at, started_at,
		ended_at, error,
		ROW_NUMBER() OVER (
			PARTITION BY task_id
			ORDER BY attempt DESC, queued_at DESC, id DESC
		) AS row_number
	FROM base_runs
),
inbox_candidates AS (
	SELECT
		c.id,
		c.identifier,
		c.scope,
		c.workspace_id,
		c.title,
		c.priority,
		c.status,
		c.owner_kind,
		c.owner_ref,
		c.latest_event_seq,
		c.approval_policy,
		c.approval_state,
		c.max_attempts,
		c.last_activity_at,
		c.priority_rank,
		lr.id AS run_id,
		lr.workspace_id AS run_workspace_id,
		lr.status AS run_status,
		lr.attempt AS run_attempt,
		lr.recovery_count AS run_recovery_count,
		lr.previous_run_id AS run_previous_run_id,
		lr.failure_kind AS run_failure_kind,
		lr.claimed_by_kind AS run_claimed_by_kind,
		lr.claimed_by_ref AS run_claimed_by_ref,
		lr.session_id AS run_session_id,
		lr.lease_until AS run_lease_until,
		lr.heartbeat_at AS run_heartbeat_at,
		lr.network_spec_json AS run_network_spec_json,
		lr.network_mode AS run_network_mode,
		lr.network_channel AS run_network_channel,
		lr.network_source AS run_network_source,
		lr.queued_at AS run_queued_at,
		lr.claimed_at AS run_claimed_at,
		lr.started_at AS run_started_at,
		lr.ended_at AS run_ended_at,
		lr.error AS run_error,
		CASE
			WHEN COALESCE(ts.archived, 0) = 1 THEN 'archived'
			WHEN c.status = 'blocked' AND c.approval_policy = 'manual' AND c.approval_state = 'pending'
				THEN 'approvals'
			WHEN lr.status = 'failed' THEN 'failed_runs'
			WHEN c.status = 'blocked' THEN 'blocked'
			WHEN c.status NOT IN ('draft', 'completed', 'failed', 'canceled') AND (
				(c.owner_kind = ? AND c.owner_ref = ?) OR EXISTS (
					SELECT 1
					FROM base_runs owned_run
					WHERE owned_run.task_id = c.id
						AND owned_run.status IN ('queued', 'claimed', 'starting', 'running')
						AND owned_run.claimed_by_kind = ?
						AND owned_run.claimed_by_ref = ?
				)
			) THEN 'my_work'
			ELSE ''
		END AS lane,
		CASE
			WHEN COALESCE(ts.archived, 0) = 1 THEN 0
			WHEN ts.task_id IS NULL THEN 1
			WHEN ts.last_seen_activity_at IS NULL OR c.last_activity_at > ts.last_seen_activity_at THEN 1
			WHEN COALESCE(ts.is_read, 0) = 0 THEN 1
			ELSE 0
		END AS is_unread,
		CASE
			WHEN COALESCE(ts.archived, 0) = 1 THEN 0
			WHEN COALESCE(ts.dismissed, 0) = 1 AND (
				ts.last_seen_activity_at IS NULL OR c.last_activity_at <= ts.last_seen_activity_at
			) THEN 1
			ELSE 0
		END AS suppressed,
		COALESCE(ts.archived, 0) AS triage_archived,
		CASE
			WHEN COALESCE(ts.archived, 0) = 1 THEN 1
			WHEN ts.task_id IS NULL THEN 0
			WHEN ts.last_seen_activity_at IS NULL OR c.last_activity_at > ts.last_seen_activity_at THEN 0
			ELSE COALESCE(ts.is_read, 0)
		END AS triage_read,
		CASE
			WHEN COALESCE(ts.archived, 0) = 1 THEN 0
			WHEN ts.task_id IS NULL THEN 0
			WHEN ts.last_seen_activity_at IS NULL OR c.last_activity_at > ts.last_seen_activity_at THEN 0
			ELSE COALESCE(ts.dismissed, 0)
		END AS triage_dismissed,
		ts.last_seen_activity_at AS triage_last_seen_activity_at,
		ts.updated_at AS triage_updated_at
	FROM catalog c
	LEFT JOIN latest_inbox_run_candidates lr ON lr.task_id = c.id AND lr.row_number = 1
	LEFT JOIN task_triage_state ts
		ON ts.task_id = c.id AND ts.actor_kind = ? AND ts.actor_id = ?
),
inbox AS (
	SELECT *
	FROM inbox_candidates
	WHERE lane <> '' AND suppressed = 0
)`

const taskInboxSelectColumns = `id, identifier, scope, workspace_id, title, priority,
	status, owner_kind, owner_ref, latest_event_seq, approval_policy, approval_state,
	max_attempts, last_activity_at, priority_rank, run_id, run_workspace_id, run_status, run_attempt,
	run_recovery_count,
	run_previous_run_id, run_failure_kind, run_claimed_by_kind, run_claimed_by_ref,
	run_session_id, run_lease_until, run_heartbeat_at, run_network_spec_json,
	run_network_mode, run_network_channel, run_network_source, run_queued_at,
	run_claimed_at, run_started_at, run_ended_at, run_error, lane, is_unread,
	triage_archived, triage_read, triage_dismissed, triage_last_seen_activity_at,
	triage_updated_at`

func taskInboxStatement(selectSQL string, baseWhere []string, where []string) string {
	base := store.AppendWhere("SELECT "+taskCatalogBaseColumns+" FROM tasks", baseWhere)
	return "WITH base_tasks AS MATERIALIZED (" + base + ")" + taskCatalogCTEBody +
		taskInboxCTESuffix + " " + store.AppendWhere(selectSQL, where)
}

func taskInboxArgs(actor taskpkg.ActorIdentity) []any {
	ownerKind := taskpkg.OwnerKindForActor(actor.Kind)
	return []any{
		string(ownerKind), actor.Ref,
		string(actor.Kind), actor.Ref,
		string(actor.Kind), actor.Ref,
	}
}

func taskInboxFilter(query taskpkg.InboxQuery) ([]string, []any) {
	where, args := store.BuildClauses(
		store.StringClause("status", string(query.Status)),
		store.StringClause("lane", string(query.Lane)),
	)
	if query.Unread != nil {
		where = append(where, "is_unread = ?")
		args = append(args, *query.Unread)
	}
	return where, args
}

func taskInboxBaseFilter(query taskpkg.InboxQuery) ([]string, []any) {
	return taskCatalogBaseFilter(taskpkg.CatalogQuery{
		Scope:         query.Scope,
		WorkspaceID:   query.WorkspaceID,
		Priority:      query.Priority,
		IncludeDrafts: true,
		OwnerKind:     query.OwnerKind,
		OwnerRef:      query.OwnerRef,
		Search:        strings.ToLower(query.Search),
	})
}

func taskInboxPageFilter(
	query taskpkg.InboxQuery,
	actor taskpkg.ActorIdentity,
	where []string,
	args []any,
) ([]string, []any, error) {
	if query.Cursor == "" {
		return where, args, nil
	}
	cursor, err := taskpkg.DecodeInboxCursor(query, actor)
	if err != nil {
		return nil, nil, err
	}
	activity := store.FormatTimestamp(cursor.ActivityAt)
	where = append(where, `(is_unread < ? OR (
		is_unread = ? AND (last_activity_at < ? OR (
			last_activity_at = ? AND (priority_rank < ? OR (
				priority_rank = ? AND id > ?
			))
		))
	))`)
	args = append(
		args,
		cursor.Unread,
		cursor.Unread,
		activity,
		activity,
		cursor.PriorityRank,
		cursor.PriorityRank,
		cursor.TaskID,
	)
	return where, args, nil
}
