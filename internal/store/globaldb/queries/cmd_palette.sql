-- name: GetCmdPaletteUsage :one
SELECT workspace_id, command_id, use_count, frecency_weight, last_used_at, updated_at
FROM cmd_palette_usage
WHERE workspace_id = sqlc.arg(workspace_id)
  AND command_id = sqlc.arg(command_id);

-- name: PutCmdPaletteUsage :exec
INSERT INTO cmd_palette_usage (
  workspace_id, command_id, use_count, frecency_weight, last_used_at, updated_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(command_id), sqlc.arg(use_count),
  sqlc.arg(frecency_weight), sqlc.arg(last_used_at), sqlc.arg(updated_at)
)
ON CONFLICT(workspace_id, command_id) DO UPDATE SET
  use_count = excluded.use_count,
  frecency_weight = excluded.frecency_weight,
  last_used_at = excluded.last_used_at,
  updated_at = excluded.updated_at;

-- name: GetCmdPaletteQueryHit :one
SELECT workspace_id, query, command_id, weight, last_used_at
FROM cmd_palette_query_hits
WHERE workspace_id = sqlc.arg(workspace_id)
  AND query = sqlc.arg(query)
  AND command_id = sqlc.arg(command_id);

-- name: PutCmdPaletteQueryHit :exec
INSERT INTO cmd_palette_query_hits (
  workspace_id, query, command_id, weight, last_used_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(query), sqlc.arg(command_id),
  sqlc.arg(weight), sqlc.arg(last_used_at)
)
ON CONFLICT(workspace_id, query, command_id) DO UPDATE SET
  weight = excluded.weight,
  last_used_at = excluded.last_used_at;

-- name: ListCmdPaletteUsage :many
SELECT workspace_id, command_id, use_count, frecency_weight, last_used_at, updated_at
FROM cmd_palette_usage
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY last_used_at DESC, command_id;

-- name: ListCmdPaletteQueryHits :many
SELECT workspace_id, query, command_id, weight, last_used_at
FROM cmd_palette_query_hits
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY query, last_used_at DESC, command_id;

-- name: ListCmdPalettePins :many
SELECT workspace_id, command_id, pinned_at
FROM cmd_palette_pins
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY pinned_at ASC, command_id;

-- name: PutCmdPalettePin :exec
INSERT INTO cmd_palette_pins (workspace_id, command_id, pinned_at)
VALUES (sqlc.arg(workspace_id), sqlc.arg(command_id), sqlc.arg(pinned_at))
ON CONFLICT(workspace_id, command_id) DO NOTHING;

-- name: DeleteCmdPalettePin :exec
DELETE FROM cmd_palette_pins
WHERE workspace_id = sqlc.arg(workspace_id)
  AND command_id = sqlc.arg(command_id);

-- name: DeleteCmdPaletteUsage :exec
DELETE FROM cmd_palette_usage
WHERE workspace_id = sqlc.arg(workspace_id)
  AND command_id = sqlc.arg(command_id);

-- name: DeleteCmdPaletteQueryHitsByCommand :exec
DELETE FROM cmd_palette_query_hits
WHERE workspace_id = sqlc.arg(workspace_id)
  AND command_id = sqlc.arg(command_id);

-- name: DeleteCmdPaletteQueryHit :exec
DELETE FROM cmd_palette_query_hits
WHERE workspace_id = sqlc.arg(workspace_id)
  AND query = sqlc.arg(query)
  AND command_id = sqlc.arg(command_id);

-- name: DeleteCmdPalettePersonalization :exec
DELETE FROM cmd_palette_usage WHERE workspace_id = sqlc.arg(workspace_id);

-- name: DeleteCmdPaletteQueryHistory :exec
DELETE FROM cmd_palette_query_hits WHERE workspace_id = sqlc.arg(workspace_id);

-- name: DeleteCmdPalettePins :exec
DELETE FROM cmd_palette_pins WHERE workspace_id = sqlc.arg(workspace_id);
