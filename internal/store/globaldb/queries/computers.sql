-- name: ListControllerComputers :many
SELECT * FROM computers WHERE account_id = ? ORDER BY name, id;

-- name: ListControllerRoutes :many
SELECT r.* FROM computer_routes r
JOIN computers c ON c.id = r.computer_id
WHERE c.account_id = ? AND r.removed = 0
ORDER BY r.position, r.id;

-- name: GetControllerRoutes :many
SELECT r.* FROM computer_routes r
JOIN computers c ON c.id = r.computer_id
WHERE c.account_id = ? AND r.computer_id = ? AND r.removed = 0
ORDER BY r.position, r.id;

-- name: AddControllerComputer :exec
INSERT INTO computers (id, account_id, name, enabled, platform, version, tls_root_sha256, weight, added_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateControllerComputer :execrows
UPDATE computers SET
    name = COALESCE(sqlc.narg(name), name),
    enabled = COALESCE(sqlc.narg(enabled), enabled),
    weight = COALESCE(sqlc.narg(weight), weight)
WHERE account_id = sqlc.arg(account_id) AND id = sqlc.arg(id);

-- name: RecordControllerConnection :exec
UPDATE computers SET last_connected_at = ?, platform = ?, version = ? WHERE account_id = ? AND id = ?;

-- name: DeleteControllerComputer :execrows
DELETE FROM computers WHERE account_id = ? AND id = ?;

-- name: AddControllerRoute :exec
INSERT INTO computer_routes (
    id, computer_id, kind, address, position, learned, ssh_user, ssh_port, ssh_alias,
    ssh_tunnel_address, ssh_service_warning
)
SELECT sqlc.arg(route_id), c.id, sqlc.arg(kind), sqlc.arg(address), sqlc.arg(position), sqlc.arg(learned),
       sqlc.narg(ssh_user), sqlc.narg(ssh_port), sqlc.narg(ssh_alias),
       sqlc.narg(ssh_tunnel_address), sqlc.narg(ssh_service_warning)
FROM computers c WHERE c.account_id = sqlc.arg(account_id) AND c.id = sqlc.arg(computer_id)
ON CONFLICT (computer_id, kind, address) DO UPDATE SET
    ssh_tunnel_address = excluded.ssh_tunnel_address, ssh_service_warning = excluded.ssh_service_warning
WHERE excluded.kind = 'ssh';

-- name: ForgetControllerRoute :execrows
UPDATE computer_routes SET removed = 1
WHERE computer_routes.id = sqlc.arg(route_id) AND computer_id = sqlc.arg(computer_id) AND learned = 1 AND removed = 0
  AND computer_id IN (SELECT c.id FROM computers c WHERE c.account_id = sqlc.arg(account_id));

-- name: PruneControllerLearnedRoutes :exec
DELETE FROM computer_routes
WHERE computer_id = sqlc.arg(computer_id) AND kind = sqlc.arg(kind) AND learned = 1
  AND address NOT IN (SELECT value FROM json_each(sqlc.arg(addresses)))
  AND computer_id IN (SELECT c.id FROM computers c WHERE c.account_id = sqlc.arg(account_id));
