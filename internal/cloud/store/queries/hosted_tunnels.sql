-- name: LockHostedTunnel :one
SELECT * FROM tunnels WHERE computer_id = $1 FOR UPDATE;

-- name: InsertHostedTunnel :one
INSERT INTO tunnels (computer_id, hostname, state, created_at)
VALUES ($1, $2, 'reserved', $3)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ReserveHostedTunnel :one
UPDATE tunnels
SET state = 'reserved', generation = generation + 1, created_at = sqlc.arg(created_at),
    cf_tunnel_id = NULL, hosted_origin = NULL, delete_after = NULL
WHERE computer_id = sqlc.arg(computer_id) AND generation = sqlc.arg(generation)
  AND state = 'deleted'
RETURNING *;

-- name: UpdateHostedTunnel :one
UPDATE tunnels
SET state = sqlc.arg(state), cf_tunnel_id = sqlc.narg(cf_tunnel_id), hosted_origin = sqlc.narg(hosted_origin)
WHERE computer_id = sqlc.arg(computer_id) AND generation = sqlc.arg(generation)
RETURNING *;

-- name: ActiveHostedTunnels :many
SELECT t.computer_id, t.hostname
FROM tunnels t JOIN computers c ON c.id = t.computer_id
WHERE c.account_id = sqlc.arg(account_id) AND c.unlinked_at IS NULL AND c.remote_enabled
  AND t.state = 'active' AND (t.delete_after IS NULL OR t.delete_after > sqlc.arg(as_of)::timestamptz);

-- name: SweepableHostedTunnels :many
SELECT t.computer_id, t.generation
FROM tunnels t
JOIN computers c ON c.id = t.computer_id
JOIN accounts a ON a.id = c.account_id
WHERE t.state <> 'deleted'
  AND (t.state IN ('deleting', 'failed') OR c.unlinked_at IS NOT NULL OR a.state <> 'active'
       OR t.delete_after <= sqlc.arg(as_of)::timestamptz
       OR (t.delete_after IS NULL AND greatest(t.created_at, c.last_control_at) <= sqlc.arg(idle_since)::timestamptz)
       OR (t.state IN ('reserved', 'provisioning') AND t.created_at <= sqlc.arg(unfinished_since)::timestamptz))
ORDER BY t.created_at, t.computer_id;

-- name: HostedTunnelCanSweep :one
SELECT EXISTS (
    SELECT 1 FROM tunnels t
    JOIN computers c ON c.id = t.computer_id
    JOIN accounts a ON a.id = c.account_id
    WHERE t.computer_id = sqlc.arg(computer_id) AND t.state <> 'deleted'
      AND (t.state IN ('deleting', 'failed') OR c.unlinked_at IS NOT NULL OR a.state <> 'active'
           OR t.delete_after <= sqlc.arg(as_of)::timestamptz
           OR (t.delete_after IS NULL AND greatest(t.created_at, c.last_control_at) <= sqlc.arg(idle_since)::timestamptz)
           OR (t.state IN ('reserved', 'provisioning') AND t.created_at <= sqlc.arg(unfinished_since)::timestamptz))
);

-- name: CountAllocatedHostedTunnels :one
SELECT count(*) FROM tunnels WHERE state <> 'deleted' AND (state <> 'failed' OR cf_tunnel_id IS NOT NULL);
