-- name: ListExtensionInputs :many
SELECT input_id, type, value_json, active, updated_at
FROM extension_inputs
WHERE extension = sqlc.arg(extension)
  AND profile = sqlc.arg(profile)
  AND workspace_id = sqlc.arg(workspace_id)
ORDER BY input_id ASC;

-- name: UpsertExtensionInput :exec
INSERT INTO extension_inputs (
  extension, profile, workspace_id, input_id, type, value_json, active, updated_at
) VALUES (
  sqlc.arg(extension), sqlc.arg(profile), sqlc.arg(workspace_id), sqlc.arg(input_id),
  sqlc.arg(type), sqlc.arg(value_json), sqlc.arg(active), sqlc.arg(updated_at)
)
ON CONFLICT(extension, profile, workspace_id, input_id) DO UPDATE SET
  type = excluded.type,
  value_json = excluded.value_json,
  active = excluded.active,
  updated_at = excluded.updated_at;

-- name: DeleteExtensionInput :exec
DELETE FROM extension_inputs
WHERE extension = sqlc.arg(extension)
  AND profile = sqlc.arg(profile)
  AND workspace_id = sqlc.arg(workspace_id)
  AND input_id = sqlc.arg(input_id);
