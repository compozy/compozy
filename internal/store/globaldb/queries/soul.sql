-- name: InsertSoulSnapshot :execrows
INSERT INTO agent_soul_snapshots (
  id, workspace_id, agent_name, source_path, digest, profile_json, body, truncated, created_at
) VALUES (
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(agent_name), sqlc.arg(source_path),
  sqlc.arg(digest), sqlc.arg(profile_json), sqlc.arg(body), sqlc.arg(truncated), sqlc.arg(created_at)
)
ON CONFLICT(workspace_id, agent_name, digest) DO NOTHING;

-- name: GetSoulSnapshot :one
SELECT id, workspace_id, agent_name, source_path, digest, profile_json, body, truncated, created_at
FROM agent_soul_snapshots WHERE id = sqlc.arg(id);

-- name: GetSoulSnapshotByDigest :one
SELECT id, workspace_id, agent_name, source_path, digest, profile_json, body, truncated, created_at
FROM agent_soul_snapshots
WHERE workspace_id = sqlc.arg(workspace_id)
  AND agent_name = sqlc.arg(agent_name)
  AND digest = sqlc.arg(digest);

-- name: InsertSoulRevision :exec
INSERT INTO agent_soul_revisions (
  id, workspace_id, agent_name, source_path, action, previous_digest, new_digest,
  body, diagnostics_json, actor_kind, actor_id, origin_kind, origin_ref, created_at
) VALUES (
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(agent_name), sqlc.arg(source_path),
  sqlc.arg(action), sqlc.arg(previous_digest), sqlc.arg(new_digest), sqlc.arg(body),
  sqlc.arg(diagnostics_json), sqlc.arg(actor_kind), sqlc.arg(actor_id),
  sqlc.arg(origin_kind), sqlc.arg(origin_ref), sqlc.arg(created_at)
);

-- name: GetSoulRevision :one
SELECT id, workspace_id, agent_name, source_path, action, previous_digest, new_digest,
       body, diagnostics_json, actor_kind, actor_id, origin_kind, origin_ref, created_at
FROM agent_soul_revisions WHERE id = sqlc.arg(id);

-- name: GetSoulRevisionForRollback :one
SELECT id, workspace_id, agent_name, source_path, action, previous_digest, new_digest,
       body, diagnostics_json, actor_kind, actor_id, origin_kind, origin_ref, created_at
FROM agent_soul_revisions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND agent_name = sqlc.arg(agent_name)
  AND id = sqlc.arg(id)
  AND action IN ('put', 'rollback');

-- name: UpdateSessionSoulSnapshot :execrows
UPDATE sessions
SET soul_snapshot_id = sqlc.narg(soul_snapshot_id), soul_digest = sqlc.arg(soul_digest),
    parent_soul_digest = sqlc.arg(parent_soul_digest), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: DeleteSoulAgentHistory :exec
DELETE FROM agent_soul_revisions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND agent_name = sqlc.arg(agent_name)
  AND source_path = sqlc.arg(source_path);
