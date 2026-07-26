-- name: InsertHeartbeatSnapshot :execrows
INSERT INTO agent_heartbeat_snapshots (
  id, workspace_id, agent_name, source_path, schema_version, digest, config_digest,
  body, frontmatter_json, resolved_json, diagnostics_json, created_at
) VALUES (
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(agent_name), sqlc.arg(source_path),
  sqlc.arg(schema_version), sqlc.arg(digest), sqlc.arg(config_digest), sqlc.arg(body),
  sqlc.arg(frontmatter_json), sqlc.arg(resolved_json), sqlc.arg(diagnostics_json), sqlc.arg(created_at)
)
ON CONFLICT(workspace_id, agent_name, digest) DO NOTHING;

-- name: GetHeartbeatSnapshot :one
SELECT id, workspace_id, agent_name, source_path, schema_version, digest, config_digest,
       body, frontmatter_json, resolved_json, diagnostics_json, created_at
FROM agent_heartbeat_snapshots WHERE id = sqlc.arg(id);

-- name: GetHeartbeatSnapshotByDigest :one
SELECT id, workspace_id, agent_name, source_path, schema_version, digest, config_digest,
       body, frontmatter_json, resolved_json, diagnostics_json, created_at
FROM agent_heartbeat_snapshots
WHERE workspace_id = sqlc.arg(workspace_id)
  AND agent_name = sqlc.arg(agent_name)
  AND digest = sqlc.arg(digest);

-- name: InsertHeartbeatRevision :exec
INSERT INTO agent_heartbeat_revisions (
  id, workspace_id, agent_name, source_path, operation, previous_digest, new_digest,
  new_snapshot_id, body, actor_kind, actor_id, created_at
) VALUES (
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(agent_name), sqlc.arg(source_path),
  sqlc.arg(operation), sqlc.narg(previous_digest), sqlc.narg(new_digest),
  sqlc.narg(new_snapshot_id), sqlc.narg(body), sqlc.arg(actor_kind),
  sqlc.arg(actor_id), sqlc.arg(created_at)
);

-- name: GetHeartbeatRevision :one
SELECT id, workspace_id, agent_name, source_path, operation, previous_digest, new_digest,
       new_snapshot_id, body, actor_kind, actor_id, created_at
FROM agent_heartbeat_revisions WHERE id = sqlc.arg(id);

-- name: GetHeartbeatRevisionForRollback :one
SELECT id, workspace_id, agent_name, source_path, operation, previous_digest, new_digest,
       new_snapshot_id, body, actor_kind, actor_id, created_at
FROM agent_heartbeat_revisions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND agent_name = sqlc.arg(agent_name)
  AND id = sqlc.arg(id)
  AND operation IN ('write', 'delete', 'rollback');

-- name: UpsertSessionHealth :exec
INSERT INTO session_health (
  session_id, workspace_id, agent_name, state, health, active_prompt, attachable,
  eligible_for_wake, ineligibility_reason, last_activity_at, last_presence_at,
  last_error, updated_at
) VALUES (
  sqlc.arg(session_id), sqlc.arg(workspace_id), sqlc.arg(agent_name), sqlc.arg(state),
  sqlc.arg(health), sqlc.arg(active_prompt), sqlc.arg(attachable), sqlc.arg(eligible_for_wake),
  sqlc.narg(ineligibility_reason), sqlc.narg(last_activity_at), sqlc.narg(last_presence_at),
  sqlc.narg(last_error), sqlc.arg(updated_at)
)
ON CONFLICT(session_id) DO UPDATE SET
  workspace_id = excluded.workspace_id, agent_name = excluded.agent_name,
  state = excluded.state, health = excluded.health, active_prompt = excluded.active_prompt,
  attachable = excluded.attachable, eligible_for_wake = excluded.eligible_for_wake,
  ineligibility_reason = excluded.ineligibility_reason,
  last_activity_at = excluded.last_activity_at, last_presence_at = excluded.last_presence_at,
  last_error = excluded.last_error, updated_at = excluded.updated_at;

-- name: GetSessionHealth :one
SELECT session_id, workspace_id, agent_name, state, health, active_prompt, attachable,
       eligible_for_wake, ineligibility_reason, last_activity_at, last_presence_at,
       last_error, updated_at
FROM session_health WHERE session_id = sqlc.arg(session_id);

-- name: MarkSessionHealthStale :execrows
UPDATE session_health
SET health = sqlc.arg(health), eligible_for_wake = 0,
    ineligibility_reason = sqlc.arg(ineligibility_reason), updated_at = sqlc.arg(updated_at)
WHERE health NOT IN ('stale', 'dead')
  AND state = 'idle'
  AND active_prompt = 0
  AND (last_presence_at IS NULL OR last_presence_at < sqlc.arg(cutoff));

-- name: UpsertHeartbeatWakeState :exec
INSERT INTO agent_heartbeat_wake_state (
  workspace_id, agent_name, session_id, policy_snapshot_id, last_wake_at, next_allowed_at,
  coalesced_count, last_result, last_reason, updated_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(agent_name), sqlc.arg(session_id), sqlc.narg(policy_snapshot_id),
  sqlc.narg(last_wake_at), sqlc.narg(next_allowed_at), sqlc.arg(coalesced_count),
  sqlc.arg(last_result), sqlc.narg(last_reason), sqlc.arg(updated_at)
)
ON CONFLICT(workspace_id, agent_name, session_id) DO UPDATE SET
  policy_snapshot_id = excluded.policy_snapshot_id, last_wake_at = excluded.last_wake_at,
  next_allowed_at = excluded.next_allowed_at, coalesced_count = excluded.coalesced_count,
  last_result = excluded.last_result, last_reason = excluded.last_reason,
  updated_at = excluded.updated_at;

-- name: GetHeartbeatWakeState :one
SELECT workspace_id, agent_name, session_id, policy_snapshot_id, last_wake_at, next_allowed_at,
       coalesced_count, last_result, last_reason, updated_at
FROM agent_heartbeat_wake_state
WHERE workspace_id = sqlc.arg(workspace_id)
  AND agent_name = sqlc.arg(agent_name)
  AND session_id = sqlc.arg(session_id);

-- name: InsertHeartbeatWakeEvent :exec
INSERT INTO agent_heartbeat_wake_events (
  id, workspace_id, agent_name, session_id, policy_snapshot_id, source, result, reason,
  synthetic_prompt_id, created_at, expires_at
) VALUES (
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(agent_name), sqlc.narg(session_id),
  sqlc.narg(policy_snapshot_id), sqlc.arg(source), sqlc.arg(result), sqlc.arg(reason),
  sqlc.narg(synthetic_prompt_id), sqlc.arg(created_at), sqlc.arg(expires_at)
);

-- name: GetHeartbeatWakeEvent :one
SELECT id, workspace_id, agent_name, session_id, policy_snapshot_id, source, result, reason,
       synthetic_prompt_id, created_at, expires_at
FROM agent_heartbeat_wake_events WHERE id = sqlc.arg(id);

-- name: SweepHeartbeatWakeEvents :execrows
DELETE FROM agent_heartbeat_wake_events
WHERE id IN (
  SELECT wake.id FROM agent_heartbeat_wake_events AS wake
  WHERE wake.expires_at < sqlc.arg(cutoff)
  ORDER BY wake.expires_at ASC, wake.id ASC
  LIMIT sqlc.arg(row_limit)
);

-- name: DeleteHeartbeatAgentHistory :exec
DELETE FROM agent_heartbeat_revisions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND agent_name = sqlc.arg(agent_name)
  AND source_path = sqlc.arg(source_path);
