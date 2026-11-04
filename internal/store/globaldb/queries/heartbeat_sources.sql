-- name: ResolveHeartbeatSource :exec
INSERT INTO agent_instance_source_states (instance_id, source_id, ref_digest, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (instance_id, source_id) DO UPDATE SET
 ref_digest = excluded.ref_digest,
 state_generation = state_generation + 1,
 committed_state = NULL, committed_at = NULL, pending_state = NULL, pending_occurrence_id = NULL,
 last_polled_at = NULL, last_success_at = NULL, last_item_count = 0,
 failure_streak = 0, last_error_code = NULL, last_error_detail = NULL, next_poll_at = NULL,
 updated_at = excluded.updated_at
WHERE ref_digest != excluded.ref_digest;

-- name: GetHeartbeatSource :one
SELECT * FROM agent_instance_source_states WHERE instance_id = ? AND source_id = ?;

-- name: ListHeartbeatSources :many
SELECT * FROM agent_instance_source_states WHERE instance_id = ? ORDER BY source_id;

-- name: DeleteRemovedHeartbeatSources :exec
DELETE FROM agent_instance_source_states WHERE instance_id = ? AND source_id NOT IN (SELECT value FROM json_each(sqlc.arg(source_ids)));

-- name: BindHeartbeatSource :execrows
UPDATE agent_instance_source_states SET pending_state = sqlc.narg(state), pending_occurrence_id = sqlc.arg(occurrence),
 state_generation = state_generation + 1, updated_at = sqlc.arg(now)
WHERE instance_id = sqlc.arg(instance_id) AND source_id = sqlc.arg(source_id)
 AND ref_digest = sqlc.arg(ref_digest) AND state_generation = sqlc.arg(generation);

-- name: CommitEmptyHeartbeatSource :execrows
UPDATE agent_instance_source_states SET committed_state = COALESCE(sqlc.narg(state), committed_state),
 committed_at = sqlc.arg(now), state_generation = state_generation + 1, updated_at = sqlc.arg(now)
WHERE instance_id = sqlc.arg(instance_id) AND source_id = sqlc.arg(source_id)
 AND ref_digest = sqlc.arg(ref_digest) AND state_generation = sqlc.arg(generation);

-- name: FinishHeartbeatSources :exec
UPDATE agent_instance_source_states SET
 committed_state = CASE WHEN sqlc.arg(completed) = 1 THEN COALESCE(pending_state, committed_state) ELSE committed_state END,
 committed_at = CASE WHEN sqlc.arg(completed) = 1 THEN sqlc.arg(now) ELSE committed_at END,
 pending_state = NULL, pending_occurrence_id = NULL, state_generation = state_generation + 1, updated_at = sqlc.arg(now)
WHERE instance_id = sqlc.arg(instance_id) AND pending_occurrence_id = sqlc.arg(occurrence);

-- name: UpdateHeartbeatSourceStatus :exec
UPDATE agent_instance_source_states SET last_polled_at = sqlc.arg(now),
 last_success_at = CASE WHEN sqlc.arg(error_code) = '' THEN sqlc.arg(now) ELSE last_success_at END,
 last_item_count = sqlc.arg(item_count), last_error_code = sqlc.arg(error_code), last_error_detail = sqlc.arg(error_detail),
 failure_streak = sqlc.arg(streak), next_poll_at = sqlc.arg(next_poll), updated_at = sqlc.arg(now)
WHERE instance_id = sqlc.arg(instance_id) AND source_id = sqlc.arg(source_id) AND ref_digest = sqlc.arg(ref_digest);

-- name: GetHeartbeatScriptApproval :one
SELECT sha256 FROM heartbeat_source_script_approvals WHERE profile_id = ? AND definition_dir = ? AND script_name = ?;

-- name: ApproveHeartbeatScript :exec
INSERT INTO heartbeat_source_script_approvals (profile_id, definition_dir, script_name, sha256, approved_at, approved_via)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (profile_id, definition_dir, script_name) DO UPDATE SET sha256 = excluded.sha256, approved_at = excluded.approved_at, approved_via = excluded.approved_via;

-- name: RevokeHeartbeatScript :exec
DELETE FROM heartbeat_source_script_approvals WHERE profile_id = ? AND definition_dir = ? AND script_name = ?;

-- name: DeferInstanceSourceWake :execrows
UPDATE agent_instance_wake_state SET next_wake_at = sqlc.arg(next_wake)
WHERE instance_id = sqlc.arg(instance_id) AND occurrence_id = '' AND next_wake_at = sqlc.arg(expected_due);
