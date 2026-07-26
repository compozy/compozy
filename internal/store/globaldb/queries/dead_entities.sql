-- name: UpsertDeadEntity :exec
INSERT INTO dead_entities (
  workspace_id, kind, entity_id, reason, marked_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(kind), sqlc.arg(entity_id), sqlc.arg(reason), sqlc.arg(marked_at)
)
ON CONFLICT(workspace_id, kind, entity_id) DO UPDATE SET
  reason = excluded.reason,
  marked_at = excluded.marked_at;

-- name: GetDeadEntity :one
SELECT workspace_id, kind, entity_id, reason, marked_at
FROM dead_entities
WHERE workspace_id = sqlc.arg(workspace_id)
  AND kind = sqlc.arg(kind)
  AND entity_id = sqlc.arg(entity_id);

-- name: DeleteDeadEntity :execrows
DELETE FROM dead_entities
WHERE workspace_id = sqlc.arg(workspace_id)
  AND kind = sqlc.arg(kind)
  AND entity_id = sqlc.arg(entity_id);

-- name: ListDeadEntities :many
SELECT workspace_id, kind, entity_id, reason, marked_at
FROM dead_entities
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY marked_at DESC, kind, entity_id;
