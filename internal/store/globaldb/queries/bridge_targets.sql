-- name: UpsertBridgeTarget :exec
INSERT INTO bridge_target_directory (
  bridge_id, canonical_route, display_name, normalized, target_type,
  qualifier, capabilities, updated_at, last_seen_at
) VALUES (
  sqlc.arg(bridge_id), sqlc.arg(canonical_route), sqlc.arg(display_name),
  sqlc.arg(normalized), sqlc.arg(target_type), sqlc.arg(qualifier),
  sqlc.arg(capabilities), sqlc.arg(updated_at), sqlc.narg(last_seen_at)
)
ON CONFLICT(bridge_id, canonical_route) DO UPDATE SET
  display_name = excluded.display_name,
  normalized = excluded.normalized,
  target_type = excluded.target_type,
  qualifier = excluded.qualifier,
  capabilities = excluded.capabilities,
  updated_at = excluded.updated_at,
  last_seen_at = excluded.last_seen_at;

-- name: UpsertBridgeTargetRefresh :exec
INSERT INTO bridge_target_directory_refresh (bridge_id, last_successful_refresh_at)
VALUES (sqlc.arg(bridge_id), sqlc.arg(last_successful_refresh_at))
ON CONFLICT(bridge_id) DO UPDATE SET
  last_successful_refresh_at = excluded.last_successful_refresh_at;

-- name: GetBridgeTargetByCanonical :one
SELECT bridge_id, canonical_route, display_name, normalized, target_type,
       qualifier, capabilities, updated_at, last_seen_at
FROM bridge_target_directory
WHERE bridge_id = sqlc.arg(bridge_id) AND canonical_route = sqlc.arg(canonical_route);

-- name: GetBridgeTargetRefresh :one
SELECT b.id,
       CAST(COALESCE(r.last_successful_refresh_at, '') AS TEXT) AS last_successful_refresh_at
FROM bridge_instances b
LEFT JOIN bridge_target_directory_refresh r ON r.bridge_id = b.id
WHERE b.id = sqlc.arg(bridge_id);
