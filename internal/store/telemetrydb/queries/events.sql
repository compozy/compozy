-- name: EventSummaryHeadSequence :one
SELECT CAST(COALESCE(MAX(seq), 0) AS INTEGER) FROM event_summaries;

-- name: InsertEventSummary :exec
INSERT INTO event_summaries (
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

-- name: DeleteEventSummariesBefore :execrows
DELETE FROM event_summaries WHERE seq IN (
  SELECT e.seq FROM event_summaries AS e WHERE e.timestamp < sqlc.arg(cutoff)
  ORDER BY e.timestamp LIMIT sqlc.arg(batch_size)
);

-- name: DeleteWorkspaceEventSummaries :execrows
DELETE FROM event_summaries WHERE seq IN (
  SELECT e.seq FROM event_summaries AS e WHERE e.workspace_id = sqlc.arg(workspace_id)
  LIMIT sqlc.arg(batch_size)
);

-- name: DeleteProfileEventSummaries :execrows
DELETE FROM event_summaries WHERE seq IN (
  SELECT e.seq FROM event_summaries AS e WHERE e.profile_id = sqlc.arg(profile_id)
  LIMIT sqlc.arg(batch_size)
);

-- name: InsertAutomationSuggestionEventSummary :exec
INSERT INTO event_summaries (
  profile_id,
  id, workspace_id, type, content_json, actor_kind, actor_id, outcome, summary, timestamp
) VALUES (
  sqlc.arg(profile_id),
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(type), sqlc.arg(content_json),
  'automation_suggestion', sqlc.arg(actor_id), sqlc.arg(outcome), sqlc.arg(summary),
  sqlc.arg(timestamp)
)
ON CONFLICT(id) DO NOTHING;


-- name: InsertEventSummaryIfAbsent :exec
INSERT INTO event_summaries (
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
) ON CONFLICT(id) DO NOTHING;

-- name: CountIdenticalEventSummary :one
SELECT COUNT(*) FROM event_summaries WHERE profile_id IS sqlc.arg(profile_id) AND id IS sqlc.arg(id) AND session_id IS sqlc.arg(session_id) AND workspace_id IS sqlc.arg(workspace_id) AND worktree_id IS sqlc.arg(worktree_id) AND type IS sqlc.arg(type) AND agent_name IS sqlc.arg(agent_name) AND content_json IS sqlc.arg(content_json) AND task_id IS sqlc.arg(task_id) AND run_id IS sqlc.arg(run_id) AND workflow_id IS sqlc.arg(workflow_id) AND claim_token_hash IS sqlc.arg(claim_token_hash) AND lease_until IS sqlc.arg(lease_until) AND coordinator_session_id IS sqlc.arg(coordinator_session_id) AND scheduler_reason IS sqlc.arg(scheduler_reason) AND hook_event IS sqlc.arg(hook_event) AND hook_name IS sqlc.arg(hook_name) AND actor_kind IS sqlc.arg(actor_kind) AND actor_id IS sqlc.arg(actor_id) AND release_reason IS sqlc.arg(release_reason) AND parent_session_id IS sqlc.arg(parent_session_id) AND root_session_id IS sqlc.arg(root_session_id) AND spawn_depth IS sqlc.arg(spawn_depth) AND provider IS sqlc.arg(provider) AND outcome IS sqlc.arg(outcome) AND summary IS sqlc.narg(summary) AND timestamp IS sqlc.arg(timestamp);
