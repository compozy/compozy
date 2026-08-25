-- name: PutCallContract :exec
INSERT INTO contract_schemas (digest, schema, created_at)
VALUES (sqlc.arg(digest), sqlc.arg(schema), sqlc.arg(created_at))
ON CONFLICT(digest) DO NOTHING;

-- name: GetCallContract :one
SELECT digest, schema FROM contract_schemas WHERE digest = sqlc.arg(digest);

-- name: PutCallPayload :exec
INSERT INTO payload_blobs (workspace_id, ref, bytes, byte_size, created_at, last_used_at)
VALUES (
  sqlc.arg(workspace_id), sqlc.arg(ref), sqlc.arg(bytes), sqlc.arg(byte_size),
  sqlc.arg(created_at), sqlc.arg(last_used_at)
)
ON CONFLICT(workspace_id, ref) DO UPDATE SET last_used_at = excluded.last_used_at;

-- name: GetCallPayload :one
SELECT workspace_id, ref, bytes, byte_size
FROM payload_blobs
WHERE workspace_id = sqlc.arg(workspace_id) AND ref = sqlc.arg(ref);
