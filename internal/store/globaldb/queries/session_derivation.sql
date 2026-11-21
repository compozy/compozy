-- name: InsertSessionDerivation :execrows
INSERT INTO session_derivations (
  workspace_id, idempotency_key, profile_id, request_fingerprint,
  source_session_id, child_session_id, kind, outcome_json, created_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(idempotency_key), sqlc.arg(profile_id), sqlc.arg(request_fingerprint),
  sqlc.arg(source_session_id), sqlc.arg(child_session_id), sqlc.arg(kind), sqlc.arg(outcome_json),
  sqlc.arg(created_at)
)
ON CONFLICT DO NOTHING;

-- name: GetSessionDerivation :one
SELECT workspace_id, idempotency_key, profile_id, request_fingerprint, source_session_id,
       child_session_id, kind, outcome_json, created_at, child_deleted_at
FROM session_derivations
WHERE workspace_id = sqlc.arg(workspace_id)
  AND idempotency_key = sqlc.arg(idempotency_key)
  AND (workspace_id <> '' OR profile_id = sqlc.arg(profile_id));

-- name: MarkSessionDerivationChildDeleted :exec
UPDATE session_derivations
SET child_deleted_at = sqlc.arg(child_deleted_at)
WHERE child_session_id = sqlc.arg(child_session_id)
  AND child_deleted_at IS NULL;

-- name: GetSessionSideAncestry :one
SELECT s.lineage_kind, s.parent_session_id
FROM sessions s
WHERE s.id = sqlc.arg(session_id)
  AND s.workspace_id = sqlc.arg(workspace_id)
  AND s.profile_id = sqlc.arg(profile_id)
UNION ALL
SELECT receipt.kind, NULL
FROM session_derivations receipt
WHERE receipt.child_session_id = sqlc.arg(session_id)
  AND receipt.workspace_id = sqlc.arg(workspace_id)
  AND receipt.profile_id = sqlc.arg(profile_id)
  AND receipt.kind = 'side'
  AND NOT EXISTS (SELECT 1 FROM sessions existing WHERE existing.id = receipt.child_session_id)
LIMIT 1;
