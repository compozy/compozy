-- name: ListMigrationNotices :many
SELECT key, payload, created_at FROM migration_notices
WHERE dismissed_at IS NULL
  AND (key <> 'gateway_removed'
       OR json_extract(payload, '$.devices') > 0
       OR json_extract(payload, '$.providers') > 0
       OR json_extract(payload, '$.connection_profiles') > 0
       OR json_extract(payload, '$.bindings_needing_confirmation') > 0)
ORDER BY created_at, key;

-- name: DismissMigrationNotice :exec
UPDATE migration_notices SET dismissed_at = sqlc.arg(dismissed_at)
WHERE key = sqlc.arg(key) AND dismissed_at IS NULL;

-- name: RecordGatewayRemovalProfiles :exec
INSERT INTO migration_notices (key, payload, created_at)
VALUES (
	'gateway_removed',
	json_object('devices', 0, 'providers', 0, 'connection_profiles', sqlc.arg(profiles),
		'bindings_needing_confirmation', 0),
	sqlc.arg(created_at)
)
ON CONFLICT(key) DO UPDATE SET
	payload = json_set(payload, '$.connection_profiles', sqlc.arg(profiles));

-- name: RefreshGatewayRemovalBindingCount :exec
UPDATE migration_notices
SET payload = json_set(payload, '$.bindings_needing_confirmation',
	(SELECT count(*) FROM ingress_bindings
	 WHERE endpoint_generation < (SELECT endpoint_generation FROM ingress_endpoint_state WHERE id = 1)))
WHERE key = 'gateway_removed' AND dismissed_at IS NULL;
