-- name: InsertEventSummaryOutbox :exec
INSERT INTO event_summary_outbox (
  profile_id,
  id, session_id, workspace_id, worktree_id, type, agent_name, content_json, task_id, run_id,
  workflow_id, claim_token_hash, lease_until, coordinator_session_id, scheduler_reason,
  hook_event, hook_name, actor_kind, actor_id, release_reason, parent_session_id,
  root_session_id, spawn_depth, provider, outcome, summary, timestamp
) VALUES (
  sqlc.arg(profile_id),
  sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(workspace_id), sqlc.arg(worktree_id), sqlc.arg(type),
  sqlc.arg(agent_name), sqlc.arg(content_json), sqlc.arg(task_id), sqlc.arg(run_id),
  sqlc.arg(workflow_id), sqlc.arg(claim_token_hash), sqlc.arg(lease_until),
  sqlc.arg(coordinator_session_id), sqlc.arg(scheduler_reason), sqlc.arg(hook_event),
  sqlc.arg(hook_name), sqlc.arg(actor_kind), sqlc.arg(actor_id), sqlc.arg(release_reason),
  sqlc.arg(parent_session_id), sqlc.arg(root_session_id), sqlc.arg(spawn_depth),
  sqlc.arg(provider), sqlc.arg(outcome), sqlc.narg(summary), sqlc.arg(timestamp)
);


-- name: ListEventSummaryOutbox :many
SELECT * FROM event_summary_outbox ORDER BY seq LIMIT sqlc.arg(batch_size);

-- name: DeleteEventSummaryOutbox :exec
DELETE FROM event_summary_outbox WHERE id IN (sqlc.slice('ids'));
