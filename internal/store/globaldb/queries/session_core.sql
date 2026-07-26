-- name: CountExpiredSessionAttachLocks :one
SELECT COUNT(1)
FROM sessions
WHERE trim(attached_to) != ''
  AND attach_expires_at IS NOT NULL
  AND attach_expires_at <= sqlc.arg(now);

-- name: SweepExpiredSessionAttachLocks :execrows
UPDATE sessions
SET attached_to = '', attach_expires_at = NULL, updated_at = sqlc.arg(now)
WHERE trim(attached_to) != ''
  AND attach_expires_at IS NOT NULL
  AND attach_expires_at <= sqlc.arg(now);

-- name: AttachSession :execrows
UPDATE sessions
SET attached_to = sqlc.arg(attached_to),
    attach_expires_at = sqlc.narg(attach_expires_at),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
  AND state = 'active'
  AND (failure_kind IS NULL OR trim(failure_kind) = '')
  AND (stall_state IS NULL OR trim(stall_state) = '')
  AND (attached_to = '' OR attach_expires_at IS NULL OR attach_expires_at <= sqlc.arg(updated_at));

-- name: GetSessionAttachState :one
SELECT state, failure_kind, stall_state, attached_to, attach_expires_at
FROM sessions
WHERE id = sqlc.arg(id);

-- name: ListSessionIDs :many
SELECT id FROM sessions;

-- name: MarkSessionOrphaned :exec
UPDATE sessions SET state = sqlc.arg(state), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: GetSessionTranscriptEpoch :one
SELECT transcript_epoch FROM sessions WHERE id = sqlc.arg(id);

-- name: EnsureSessionTranscriptEpoch :one
UPDATE sessions
SET transcript_epoch = sqlc.arg(minimum), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND transcript_epoch < sqlc.arg(minimum)
RETURNING transcript_epoch;

-- name: ListSessionHealthByIDs :many
SELECT session_id, workspace_id, agent_name, state, health, active_prompt, attachable,
       eligible_for_wake, ineligibility_reason, last_activity_at, last_presence_at,
       last_error, updated_at
FROM session_health
WHERE session_id IN (sqlc.slice(session_ids))
ORDER BY updated_at DESC, session_id DESC;

-- name: UpsertSession :execrows
INSERT INTO sessions (
  id, name, agent_name, provider, workspace_id, session_type,
  network_spec_json, network_mode, network_channel, network_source, state,
  parent_session_id, root_session_id, spawn_depth, spawn_role, ttl_expires_at,
  auto_stop_on_parent, spawn_budget_json, permission_policy_json,
  acp_session_id, stop_reason, stop_detail, failure_kind, failure_summary, crash_bundle_path,
  subprocess_pid, subprocess_started_at, last_update_at, stall_state, stall_reason, activity_json,
  transcript_epoch, soul_snapshot_id, soul_digest, parent_soul_digest,
  sandbox_id, sandbox_backend, sandbox_profile, sandbox_instance_id,
  sandbox_state, sandbox_provider_state_json, sandbox_last_sync_at, sandbox_last_sync_error,
  created_at, updated_at
) VALUES (
  sqlc.arg(id), sqlc.narg(name), sqlc.arg(agent_name), sqlc.arg(provider), sqlc.arg(workspace_id),
  sqlc.arg(session_type), sqlc.arg(network_spec_json), sqlc.arg(network_mode),
  sqlc.narg(network_channel), sqlc.arg(network_source), sqlc.arg(state), sqlc.narg(parent_session_id),
  sqlc.narg(root_session_id), sqlc.arg(spawn_depth), sqlc.narg(spawn_role), sqlc.narg(ttl_expires_at),
  sqlc.arg(auto_stop_on_parent), sqlc.arg(spawn_budget_json), sqlc.arg(permission_policy_json),
  sqlc.narg(acp_session_id), sqlc.narg(stop_reason), sqlc.narg(stop_detail), sqlc.narg(failure_kind),
  sqlc.arg(failure_summary), sqlc.arg(crash_bundle_path), sqlc.arg(subprocess_pid),
  sqlc.narg(subprocess_started_at), sqlc.narg(last_update_at), sqlc.arg(stall_state),
  sqlc.arg(stall_reason), sqlc.arg(activity_json), sqlc.arg(transcript_epoch),
  sqlc.narg(soul_snapshot_id), sqlc.arg(soul_digest), sqlc.arg(parent_soul_digest),
  sqlc.arg(sandbox_id), sqlc.arg(sandbox_backend), sqlc.arg(sandbox_profile),
  sqlc.arg(sandbox_instance_id), sqlc.arg(sandbox_state), sqlc.arg(sandbox_provider_state_json),
  sqlc.narg(sandbox_last_sync_at), sqlc.arg(sandbox_last_sync_error),
  sqlc.arg(created_at), sqlc.arg(updated_at)
)
ON CONFLICT(id) DO UPDATE SET
  name = excluded.name,
  agent_name = excluded.agent_name,
  provider = excluded.provider,
  workspace_id = excluded.workspace_id,
  session_type = excluded.session_type,
  state = excluded.state,
  parent_session_id = excluded.parent_session_id,
  root_session_id = excluded.root_session_id,
  spawn_depth = excluded.spawn_depth,
  spawn_role = excluded.spawn_role,
  ttl_expires_at = excluded.ttl_expires_at,
  auto_stop_on_parent = excluded.auto_stop_on_parent,
  spawn_budget_json = excluded.spawn_budget_json,
  permission_policy_json = excluded.permission_policy_json,
  acp_session_id = excluded.acp_session_id,
  stop_reason = excluded.stop_reason,
  stop_detail = excluded.stop_detail,
  failure_kind = excluded.failure_kind,
  failure_summary = excluded.failure_summary,
  crash_bundle_path = excluded.crash_bundle_path,
  subprocess_pid = excluded.subprocess_pid,
  subprocess_started_at = excluded.subprocess_started_at,
  last_update_at = excluded.last_update_at,
  stall_state = excluded.stall_state,
  stall_reason = excluded.stall_reason,
  activity_json = excluded.activity_json,
  transcript_epoch = MAX(sessions.transcript_epoch, excluded.transcript_epoch),
  soul_snapshot_id = excluded.soul_snapshot_id,
  soul_digest = excluded.soul_digest,
  parent_soul_digest = excluded.parent_soul_digest,
  sandbox_id = excluded.sandbox_id,
  sandbox_backend = excluded.sandbox_backend,
  sandbox_profile = excluded.sandbox_profile,
  sandbox_instance_id = excluded.sandbox_instance_id,
  sandbox_state = excluded.sandbox_state,
  sandbox_provider_state_json = excluded.sandbox_provider_state_json,
  sandbox_last_sync_at = excluded.sandbox_last_sync_at,
  sandbox_last_sync_error = excluded.sandbox_last_sync_error,
  updated_at = excluded.updated_at
WHERE sessions.network_spec_json IS excluded.network_spec_json
  AND sessions.network_mode IS excluded.network_mode
  AND sessions.network_channel IS excluded.network_channel
  AND sessions.network_source IS excluded.network_source;
