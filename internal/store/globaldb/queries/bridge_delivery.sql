-- name: BridgeDeliveryExists :one
SELECT EXISTS (
  SELECT 1 FROM bridge_deliveries WHERE delivery_id = sqlc.arg(delivery_id)
) AS found;

-- name: InsertBridgeDelivery :exec
INSERT INTO bridge_deliveries (
  delivery_id, session_id, turn_id, routing_key, bridge_instance_id,
  scope, workspace_id, state, last_sent_seq, last_acked_seq,
  remote_message_id, terminal_error, created_at, updated_at
) VALUES (
  sqlc.arg(delivery_id), sqlc.arg(session_id), sqlc.arg(turn_id), sqlc.arg(routing_key),
  sqlc.arg(bridge_instance_id), sqlc.arg(scope), sqlc.narg(workspace_id), sqlc.arg(state),
  sqlc.arg(last_sent_seq), sqlc.arg(last_acked_seq), sqlc.narg(remote_message_id),
  sqlc.narg(terminal_error), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: GetBridgeDelivery :one
SELECT
  delivery_id, session_id, turn_id, routing_key, bridge_instance_id,
  scope, workspace_id, state, last_sent_seq, last_acked_seq,
  remote_message_id, terminal_error, created_at, updated_at
FROM bridge_deliveries
WHERE delivery_id = sqlc.arg(delivery_id);

-- name: UpdateBridgeDeliveryCheckpoint :exec
UPDATE bridge_deliveries SET
  state = sqlc.arg(state),
  last_sent_seq = sqlc.arg(last_sent_seq),
  last_acked_seq = sqlc.arg(last_acked_seq),
  remote_message_id = sqlc.narg(remote_message_id),
  terminal_error = sqlc.narg(terminal_error),
  updated_at = sqlc.arg(updated_at)
WHERE delivery_id = sqlc.arg(delivery_id);

-- name: GetBridgeDeliveryInstanceScope :one
SELECT scope, workspace_id
FROM bridge_instances
WHERE id = sqlc.arg(bridge_instance_id);

-- name: UpsertBridgeDeliveryMetrics :exec
INSERT INTO bridge_delivery_metrics (
  bridge_instance_id, scope, workspace_id, delivery_dropped_total,
  delivery_dropped_by_reason_json, delivery_failures_total,
  last_error, last_error_at, last_success_at, updated_at
) VALUES (
  sqlc.arg(bridge_instance_id), sqlc.arg(scope), sqlc.narg(workspace_id),
  sqlc.arg(delivery_dropped_total), sqlc.arg(delivery_dropped_by_reason_json),
  sqlc.arg(delivery_failures_total), sqlc.narg(last_error), sqlc.narg(last_error_at),
  sqlc.narg(last_success_at), sqlc.arg(updated_at)
)
ON CONFLICT(bridge_instance_id) DO UPDATE SET
  delivery_dropped_total = excluded.delivery_dropped_total,
  delivery_dropped_by_reason_json = excluded.delivery_dropped_by_reason_json,
  delivery_failures_total = excluded.delivery_failures_total,
  last_error = excluded.last_error,
  last_error_at = excluded.last_error_at,
  last_success_at = excluded.last_success_at,
  updated_at = excluded.updated_at;

-- name: GetBridgeDeliveryMetric :one
SELECT
  bridge_instance_id, scope, workspace_id, delivery_dropped_total,
  delivery_dropped_by_reason_json, delivery_failures_total,
  last_error, last_error_at, last_success_at, updated_at
FROM bridge_delivery_metrics
WHERE bridge_instance_id = sqlc.arg(bridge_instance_id);
