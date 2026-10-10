-- name: ReserveSubagent :execrows
INSERT INTO session_subagents (isolation, id, workspace_id, parent_session_id, parent_turn_id, parent_tool_call_id, child_session_id, origin, provider_tool_call_id, idempotency_key, request_fingerprint, title, role, task_chars, pending_task, runtime_agent, runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed, depth, status, work_state, progress, result, result_truncated, error, wake_policy, delivery, wake_message_id, acknowledged_turn_id, started_at, settled_at, created_at, updated_at)
VALUES (sqlc.arg(isolation), sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(parent_session_id), sqlc.arg(parent_turn_id), sqlc.arg(parent_tool_call_id), sqlc.narg(child_session_id), sqlc.arg(origin), sqlc.arg(provider_tool_call_id), sqlc.arg(idempotency_key), sqlc.arg(request_fingerprint), sqlc.arg(title), sqlc.arg(role), sqlc.arg(task_chars), sqlc.narg(pending_task), sqlc.arg(runtime_agent), sqlc.arg(runtime_provider), sqlc.arg(runtime_model), sqlc.arg(runtime_reasoning_effort), sqlc.arg(runtime_speed), sqlc.arg(depth), sqlc.arg(status), sqlc.arg(work_state), sqlc.arg(progress), sqlc.narg(result), sqlc.arg(result_truncated), sqlc.narg(error), sqlc.arg(wake_policy), sqlc.arg(delivery), sqlc.narg(wake_message_id), sqlc.arg(acknowledged_turn_id), sqlc.narg(started_at), sqlc.narg(settled_at), sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT(parent_session_id,idempotency_key) DO NOTHING;

-- name: GetSubagent :one
SELECT * FROM session_subagents WHERE id = ?;

-- name: GetSubagentByKey :one
SELECT * FROM session_subagents WHERE parent_session_id = ? AND idempotency_key = ?;

-- name: GetSubagentByChild :one
SELECT * FROM session_subagents WHERE child_session_id = ?;

-- name: LinkSubagentChild :execrows
UPDATE session_subagents SET child_session_id = sqlc.arg(child_id), status = 'running', started_at = sqlc.arg(now), updated_at = sqlc.arg(now) WHERE id = sqlc.arg(id) AND status = 'queued';

-- name: UpdateSubagentProgress :execrows
UPDATE session_subagents SET progress = ?, updated_at = ? WHERE id = ? AND status IN ('queued','running','waiting');

-- name: FinalizeSubagent :execrows
UPDATE session_subagents SET status = sqlc.arg(status), work_state = sqlc.arg(work_state), result = sqlc.narg(result), error = sqlc.narg(error), result_truncated = sqlc.arg(result_truncated), settled_at = sqlc.arg(now), updated_at = sqlc.arg(now) WHERE id = sqlc.arg(id) AND status IN ('queued','running','waiting');

-- name: ListSubagents :many
SELECT * FROM session_subagents
WHERE (sqlc.arg(workspace) = '' OR workspace_id = sqlc.arg(workspace))
 AND (sqlc.arg(parent) = '' OR parent_session_id = sqlc.arg(parent))
 AND (sqlc.arg(origins) = '[]' OR origin IN (SELECT value FROM json_each(sqlc.arg(origins))))
 AND (sqlc.arg(statuses) = '[]' OR status IN (SELECT value FROM json_each(sqlc.arg(statuses))))
 AND (sqlc.arg(cursor_id) = '' OR (created_at,id) < (sqlc.arg(cursor_time),sqlc.arg(cursor_id)))
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg(page_limit);

-- name: SubagentSummaries :many
SELECT parent_session_id, count(*) AS total,
 sum(CASE WHEN status IN ('queued','running','waiting') THEN 1 ELSE 0 END) AS live,
 sum(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed,
 sum(CASE WHEN status = 'waiting' THEN 1 ELSE 0 END) AS attention
FROM session_subagents WHERE parent_session_id IN (SELECT value FROM json_each(sqlc.arg(parent_ids))) GROUP BY parent_session_id;

-- name: GetOpenSubagentWake :one
SELECT * FROM session_subagent_wakes WHERE parent_session_id = ? AND state = 'open';

-- name: GetSubagentWake :one
SELECT * FROM session_subagent_wakes WHERE wake_message_id = ?;

-- name: InsertSubagentWake :exec
INSERT INTO session_subagent_wakes(wake_message_id,workspace_id,parent_session_id,state,route,created_at,updated_at) SELECT sqlc.arg(wake_id),workspace_id,id,'open','queue',sqlc.arg(now),sqlc.arg(now) FROM sessions WHERE id = sqlc.arg(parent_id);

-- name: ClaimSubagentWakeRows :exec
UPDATE session_subagents SET delivery = 'claimed',wake_message_id = sqlc.arg(wake_id),updated_at = sqlc.arg(now) WHERE parent_session_id = sqlc.arg(parent_id) AND id IN (SELECT value FROM json_each(sqlc.arg(ids))) AND status IN ('completed','failed','canceled','interrupted') AND delivery IN ('none','pending');

-- name: ListSubagentWakeRows :many
SELECT * FROM session_subagents WHERE wake_message_id = ? ORDER BY created_at,id;

-- name: SetSubagentWakeInput :execrows
UPDATE session_subagent_wakes SET route = ?,input_entry_id = ?,updated_at = ? WHERE wake_message_id = ? AND state = 'open';

-- name: MarkSubagentWakeSteerRequeued :execrows
UPDATE session_subagent_wakes SET steer_requeued = 1,state = 'open',route = 'queue',updated_at = ? WHERE wake_message_id = ? AND state IN ('open','dispatched') AND steer_requeued = 0;

-- name: MarkSubagentWakeDispatched :execrows
UPDATE session_subagent_wakes SET state = 'dispatched',updated_at = ? WHERE wake_message_id = ? AND state = 'open';

-- name: SettleSubagentWake :execrows
UPDATE session_subagent_wakes SET state = ?,updated_at = ? WHERE wake_message_id = ? AND state IN ('open','dispatched');

-- name: SettleSubagentWakeRows :many
UPDATE session_subagents SET delivery = sqlc.arg(delivery),wake_message_id = CASE WHEN sqlc.arg(delivery) = 'pending' AND (SELECT attempts FROM session_subagent_wakes WHERE session_subagent_wakes.wake_message_id = sqlc.arg(wake_id)) = 0 THEN NULL ELSE wake_message_id END,updated_at = sqlc.arg(now) WHERE wake_message_id = sqlc.arg(wake_id) AND delivery = 'claimed' RETURNING *;

-- name: SetSubagentsPending :exec
UPDATE session_subagents SET delivery = 'pending',wake_message_id = NULL,updated_at = ? WHERE id IN (SELECT value FROM json_each(sqlc.arg(ids))) AND status IN ('completed','failed','canceled','interrupted') AND delivery = 'none';

-- name: AcknowledgeSubagent :exec
UPDATE session_subagents SET delivery = 'acknowledged',acknowledged_turn_id = sqlc.arg(turn_id),wake_message_id = CASE WHEN CAST(sqlc.arg(remove_wake) AS INTEGER) = 1 THEN NULL ELSE wake_message_id END,updated_at = sqlc.arg(now) WHERE id = sqlc.arg(id) AND status IN ('completed','failed','canceled','interrupted') AND delivery NOT IN ('acknowledged','disposed');

-- name: CancelEmptySubagentWake :exec
UPDATE session_subagent_wakes SET state = 'canceled',updated_at = ? WHERE session_subagent_wakes.wake_message_id = sqlc.arg(wake_id) AND state = 'open' AND NOT EXISTS (SELECT 1 FROM session_subagents WHERE wake_message_id = sqlc.arg(wake_id) AND delivery = 'claimed');

-- name: DisposeSubagents :many
UPDATE session_subagents SET delivery = 'disposed',wake_message_id = NULL,updated_at = sqlc.arg(now) WHERE origin = 'delegated' AND parent_session_id = sqlc.arg(parent_id) AND (sqlc.arg(turn_id) = '' OR parent_turn_id = sqlc.arg(turn_id)) AND (sqlc.arg(ids) = '[]' OR id IN (SELECT value FROM json_each(sqlc.arg(ids)))) AND delivery IN ('none','pending','claimed') RETURNING *;

-- name: CancelEmptyParentSubagentWakes :exec
UPDATE session_subagent_wakes SET state = 'canceled',updated_at = ? WHERE session_subagent_wakes.parent_session_id = ? AND state = 'open' AND NOT EXISTS (SELECT 1 FROM session_subagents WHERE session_subagents.wake_message_id = session_subagent_wakes.wake_message_id AND delivery = 'claimed');

-- name: UpgradeSubagentWakePolicy :exec
UPDATE session_subagents SET wake_policy = 'always',updated_at = ? WHERE id = ? AND wake_policy = 'settled_only';

-- name: ListStaleReservedSubagents :many
SELECT * FROM session_subagents WHERE origin = 'delegated' AND status = 'queued' AND created_at < ? ORDER BY created_at,id;

-- name: ListUnfinalizedDelegatedSubagents :many
SELECT * FROM session_subagents WHERE origin = 'delegated' AND status IN ('queued','running','waiting') ORDER BY created_at,id;

-- name: ListOpenSubagentWakes :many
SELECT * FROM session_subagent_wakes WHERE state IN ('open','dispatched') ORDER BY created_at,wake_message_id;

-- name: ListPendingSubagents :many
SELECT * FROM session_subagents WHERE delivery = 'pending' ORDER BY created_at,id;

-- name: ListOrphanSubagentSessions :many
SELECT id FROM sessions WHERE spawn_role = 'subagent' AND state <> 'stopped' AND NOT EXISTS (SELECT 1 FROM session_subagents WHERE child_session_id = sessions.id) ORDER BY id;


-- name: MarkSubagentFirstPromptAdmitted :execrows
UPDATE session_subagents SET pending_task = NULL, updated_at = ? WHERE id = ?;

-- name: DeleteReservedSubagent :execrows
DELETE FROM session_subagents WHERE id = ? AND status = 'queued' AND child_session_id IS NULL;
-- name: ListSubagentArchiveFamily :many
WITH RECURSIVE family(id,state) AS (
 SELECT sessions.id,sessions.state FROM sessions WHERE sessions.workspace_id = sqlc.arg(workspace_id) AND sessions.id = sqlc.arg(session_id)
 UNION
 SELECT child.id,child.state FROM sessions child JOIN family parent ON child.parent_session_id = parent.id
 WHERE child.workspace_id = sqlc.arg(workspace_id) AND child.spawn_role = 'subagent'
)
SELECT id,state FROM family ORDER BY id;

-- name: SetSubagentFamilyArchived :exec
UPDATE sessions SET archived_at = sqlc.narg(archived_at),updated_at = sqlc.arg(now)
WHERE workspace_id = sqlc.arg(workspace_id) AND id IN (SELECT value FROM json_each(sqlc.arg(ids)))
 AND ((sqlc.narg(archived_at) IS NULL AND archived_at IS NOT NULL) OR (sqlc.narg(archived_at) IS NOT NULL AND archived_at IS NULL));

-- name: UpdateSubagentState :execrows
UPDATE session_subagents
SET status = sqlc.arg(status), work_state = sqlc.arg(work_state), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('queued','running','waiting')
 AND (status <> sqlc.arg(status) OR work_state <> sqlc.arg(work_state));

-- name: RewriteSubagentWakeInput :execrows
UPDATE session_input_queue
SET text = sqlc.arg(text),
 synthetic_prompt_json = json_set(synthetic_prompt_json, '$.metadata', json(sqlc.arg(metadata))),
 updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(input_id) AND session_id = sqlc.arg(parent_id)
 AND status = 'queued' AND owner_kind = 'synthetic'
 AND json_type(synthetic_prompt_json) = 'object';

-- name: UpdateNativeSubagentTitle :execrows
UPDATE session_subagents SET title = @title, updated_at = @updated_at
WHERE id = @id AND origin = 'provider_native' AND title <> @title;

-- name: ListSubagentWakesByParent :many
SELECT * FROM session_subagent_wakes
WHERE parent_session_id = sqlc.arg(parent_id)
 AND (sqlc.arg(states) = '[]' OR state IN (SELECT value FROM json_each(sqlc.arg(states))))
ORDER BY created_at,wake_message_id;

-- name: ListUnfinalizedNativeSubagents :many
SELECT * FROM session_subagents WHERE origin = 'provider_native' AND status IN ('queued','running','waiting') ORDER BY parent_session_id,id;

-- name: InheritSubagentWakeAttempts :exec
UPDATE session_subagent_wakes SET attempts = MAX(attempts, COALESCE((
 SELECT MAX(prior.attempts) FROM session_subagents child JOIN session_subagent_wakes prior ON prior.wake_message_id = child.wake_message_id
 WHERE child.parent_session_id = sqlc.arg(parent_id) AND child.id IN (SELECT value FROM json_each(sqlc.arg(ids)))
),0)) WHERE session_subagent_wakes.wake_message_id = sqlc.arg(wake_id);

-- name: FailSubagentWake :execrows
UPDATE session_subagent_wakes SET attempts = attempts + 1,state = 'canceled',updated_at = ? WHERE wake_message_id = ? AND state IN ('open','dispatched');

-- name: AssociateSubagentWorktree :execrows
UPDATE session_subagents SET worktree_id = sqlc.narg(worktree_id), worktree_name = sqlc.narg(worktree_name), worktree_branch = sqlc.narg(worktree_branch), worktree_base_ref = sqlc.narg(worktree_base_ref), worktree_base_sha = sqlc.narg(worktree_base_sha), worktree_path = sqlc.narg(worktree_path) WHERE id = sqlc.arg(id) AND isolation = 'worktree';

-- name: SetSubagentWorktreeCleanup :execrows
UPDATE session_subagents SET worktree_cleanup = sqlc.narg(worktree_cleanup) WHERE id = sqlc.arg(id);

-- name: ListSubagentWorktreeCleanupPending :many
SELECT * FROM session_subagents WHERE worktree_cleanup = 'pending' ORDER BY created_at,id;

-- name: SetSubagentWorktreeFacts :exec
UPDATE session_subagents SET git_head_sha = sqlc.narg(git_head_sha), git_commits_ahead = sqlc.narg(git_commits_ahead), git_dirty_files = sqlc.narg(git_dirty_files), git_observed_at = sqlc.narg(git_observed_at), pr_status = sqlc.narg(pr_status), pr_url = sqlc.narg(pr_url), pr_number = sqlc.narg(pr_number) WHERE id = sqlc.arg(id) AND isolation = 'worktree' AND pr_status IS NULL AND status IN ('completed','failed','canceled','interrupted');

-- name: HasSubagentCommittedAdmission :one
SELECT EXISTS(SELECT 1 FROM session_prompt_admissions WHERE workspace_id = sqlc.arg(workspace_id) AND session_id = sqlc.arg(session_id) AND idempotency_key = sqlc.arg(subagent_id) AND state IN ('dispatch_committed','indeterminate','completed'));
