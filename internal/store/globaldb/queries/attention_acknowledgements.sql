-- name: ListAcknowledgedAttention :many
SELECT occurrence_id FROM attention_acknowledgements
WHERE profile_id = sqlc.arg(profile_id)
  AND actor_kind = sqlc.arg(actor_kind)
  AND actor_id = sqlc.arg(actor_id)
  AND occurrence_id IN (SELECT CAST(value AS TEXT) FROM json_each(sqlc.arg(occurrence_ids)));

-- name: SaveAttentionSnapshot :exec
INSERT INTO attention_snapshots (id, profile_id, actor_kind, actor_id, population, occurrence_ids, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET expires_at = excluded.expires_at;

-- name: DeleteExpiredAttentionSnapshots :exec
DELETE FROM attention_snapshots WHERE expires_at < sqlc.arg(now);

-- name: GetAttentionSnapshot :one
SELECT occurrence_ids FROM attention_snapshots
WHERE id = sqlc.arg(id) AND profile_id = sqlc.arg(profile_id)
  AND actor_kind = sqlc.arg(actor_kind) AND actor_id = sqlc.arg(actor_id)
  AND population = sqlc.arg(population) AND expires_at >= sqlc.arg(now);

-- name: AcknowledgeAttentionOccurrences :exec
INSERT INTO attention_acknowledgements (profile_id, actor_kind, actor_id, occurrence_id, acknowledged_at)
SELECT sqlc.arg(profile_id), sqlc.arg(actor_kind), sqlc.arg(actor_id), CAST(value AS TEXT), sqlc.arg(now)
FROM json_each(sqlc.arg(occurrence_ids)) WHERE true
ON CONFLICT(profile_id, actor_kind, actor_id, occurrence_id) DO NOTHING;
