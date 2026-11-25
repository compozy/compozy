-- name: GetIngressEndpointState :one
SELECT * FROM ingress_endpoint_state WHERE id = 1;

-- name: UpsertIngressEndpointState :exec
INSERT INTO ingress_endpoint_state (id, endpoint_url, endpoint_generation, reachable, updated_at)
VALUES (1, sqlc.arg(endpoint_url), 1, sqlc.arg(reachable), sqlc.arg(updated_at))
ON CONFLICT(id) DO UPDATE SET
    endpoint_url = excluded.endpoint_url,
    endpoint_generation = ingress_endpoint_state.endpoint_generation +
        (ingress_endpoint_state.endpoint_url <> excluded.endpoint_url),
    reachable = excluded.reachable,
    updated_at = excluded.updated_at;
