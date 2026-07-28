-- name: InsertBridgeInstance :exec
INSERT INTO bridge_instances (
  id, scope, workspace_id, platform, extension_name, display_name,
  source, enabled, status, dm_policy, routing_policy, provider_config,
  delivery_defaults, notification_suppress, degradation_reason, degradation_message,
  created_at, updated_at
) VALUES (
  sqlc.arg(id), sqlc.arg(scope), sqlc.narg(workspace_id), sqlc.arg(platform),
  sqlc.arg(extension_name), sqlc.arg(display_name), sqlc.arg(source), sqlc.arg(enabled),
  sqlc.arg(status), sqlc.arg(dm_policy), sqlc.arg(routing_policy), sqlc.narg(provider_config),
  sqlc.narg(delivery_defaults), sqlc.arg(notification_suppress), sqlc.narg(degradation_reason),
  sqlc.narg(degradation_message), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: UpsertBridgeInstance :exec
INSERT INTO bridge_instances (
  id, scope, workspace_id, platform, extension_name, display_name,
  source, enabled, status, dm_policy, routing_policy, provider_config,
  delivery_defaults, notification_suppress, degradation_reason, degradation_message,
  created_at, updated_at
) VALUES (
  sqlc.arg(id), sqlc.arg(scope), sqlc.narg(workspace_id), sqlc.arg(platform),
  sqlc.arg(extension_name), sqlc.arg(display_name), sqlc.arg(source), sqlc.arg(enabled),
  sqlc.arg(status), sqlc.arg(dm_policy), sqlc.arg(routing_policy), sqlc.narg(provider_config),
  sqlc.narg(delivery_defaults), sqlc.arg(notification_suppress), sqlc.narg(degradation_reason),
  sqlc.narg(degradation_message), sqlc.arg(created_at), sqlc.arg(updated_at)
)
ON CONFLICT(id) DO UPDATE SET
  scope = excluded.scope, workspace_id = excluded.workspace_id, platform = excluded.platform,
  extension_name = excluded.extension_name, display_name = excluded.display_name,
  source = excluded.source, enabled = excluded.enabled, status = excluded.status,
  dm_policy = excluded.dm_policy, routing_policy = excluded.routing_policy,
  provider_config = excluded.provider_config, delivery_defaults = excluded.delivery_defaults,
  notification_suppress = excluded.notification_suppress,
  degradation_reason = excluded.degradation_reason,
  degradation_message = excluded.degradation_message, updated_at = excluded.updated_at;

-- name: UpdateBridgeInstance :execrows
UPDATE bridge_instances SET
  scope = sqlc.arg(scope), workspace_id = sqlc.narg(workspace_id), platform = sqlc.arg(platform),
  extension_name = sqlc.arg(extension_name), display_name = sqlc.arg(display_name),
  source = sqlc.arg(source), enabled = sqlc.arg(enabled), status = sqlc.arg(status),
  dm_policy = sqlc.arg(dm_policy), routing_policy = sqlc.arg(routing_policy),
  provider_config = sqlc.narg(provider_config), delivery_defaults = sqlc.narg(delivery_defaults),
  notification_suppress = sqlc.arg(notification_suppress),
  degradation_reason = sqlc.narg(degradation_reason), degradation_message = sqlc.narg(degradation_message),
  updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: DeleteBridgeInstance :execrows
DELETE FROM bridge_instances WHERE id = sqlc.arg(id);

-- name: GetBridgeInstance :one
SELECT id, scope, workspace_id, platform, extension_name, display_name,
       source, enabled, status, dm_policy, routing_policy, provider_config,
       delivery_defaults, notification_suppress, degradation_reason, degradation_message,
       created_at, updated_at
FROM bridge_instances WHERE id = sqlc.arg(id);

-- name: ListBridgeInstances :many
SELECT id, scope, workspace_id, platform, extension_name, display_name,
       source, enabled, status, dm_policy, routing_policy, provider_config,
       delivery_defaults, notification_suppress, degradation_reason, degradation_message,
       created_at, updated_at
FROM bridge_instances
ORDER BY display_name ASC, created_at ASC, id ASC;

-- name: ListBridgeInstanceIDs :many
SELECT id FROM bridge_instances;

-- name: UpsertBridgeSecretBinding :exec
INSERT INTO bridge_secret_bindings (
  bridge_instance_id, binding_name, secret_ref, kind, created_at, updated_at
) VALUES (
  sqlc.arg(bridge_instance_id), sqlc.arg(binding_name), sqlc.arg(secret_ref),
  sqlc.arg(kind), sqlc.arg(created_at), sqlc.arg(updated_at)
)
ON CONFLICT(bridge_instance_id, binding_name) DO UPDATE SET
  secret_ref = excluded.secret_ref, kind = excluded.kind, updated_at = excluded.updated_at;

-- name: GetBridgeSecretBinding :one
SELECT bridge_instance_id, binding_name, secret_ref, kind, created_at, updated_at
FROM bridge_secret_bindings
WHERE bridge_instance_id = sqlc.arg(bridge_instance_id) AND binding_name = sqlc.arg(binding_name);

-- name: ListBridgeSecretBindings :many
SELECT bridge_instance_id, binding_name, secret_ref, kind, created_at, updated_at
FROM bridge_secret_bindings WHERE bridge_instance_id = sqlc.arg(bridge_instance_id)
ORDER BY binding_name ASC;

-- name: DeleteBridgeSecretBinding :execrows
DELETE FROM bridge_secret_bindings
WHERE bridge_instance_id = sqlc.arg(bridge_instance_id) AND binding_name = sqlc.arg(binding_name);

-- name: UpsertBridgeRoute :exec
INSERT INTO bridge_routes (
  routing_key_hash, scope, workspace_id, bridge_instance_id, peer_id,
  thread_id, group_id, session_id, agent_name, last_activity_at, created_at, updated_at
) VALUES (
  sqlc.arg(routing_key_hash), sqlc.arg(scope), sqlc.narg(workspace_id),
  sqlc.arg(bridge_instance_id), sqlc.narg(peer_id), sqlc.narg(thread_id),
  sqlc.narg(group_id), sqlc.arg(session_id), sqlc.arg(agent_name),
  sqlc.arg(last_activity_at), sqlc.arg(created_at), sqlc.arg(updated_at)
)
ON CONFLICT(routing_key_hash) DO UPDATE SET
  scope = excluded.scope, workspace_id = excluded.workspace_id,
  bridge_instance_id = excluded.bridge_instance_id, peer_id = excluded.peer_id,
  thread_id = excluded.thread_id, group_id = excluded.group_id,
  session_id = excluded.session_id, agent_name = excluded.agent_name,
  last_activity_at = excluded.last_activity_at, updated_at = excluded.updated_at;

-- name: GetBridgeRoute :one
SELECT routing_key_hash, scope, workspace_id, bridge_instance_id, peer_id,
       thread_id, group_id, session_id, agent_name, last_activity_at, created_at, updated_at
FROM bridge_routes WHERE routing_key_hash = sqlc.arg(routing_key_hash);

-- name: ListBridgeRoutes :many
SELECT routing_key_hash, scope, workspace_id, bridge_instance_id, peer_id,
       thread_id, group_id, session_id, agent_name, last_activity_at, created_at, updated_at
FROM bridge_routes WHERE bridge_instance_id = sqlc.arg(bridge_instance_id)
ORDER BY updated_at DESC, created_at DESC, routing_key_hash ASC;

-- name: DeleteBridgeRoute :execrows
DELETE FROM bridge_routes WHERE routing_key_hash = sqlc.arg(routing_key_hash);

-- name: UpsertBridgeIngestDedup :exec
INSERT INTO bridge_ingest_dedup (idempotency_key, bridge_instance_id, received_at, expires_at)
VALUES (sqlc.arg(idempotency_key), sqlc.arg(bridge_instance_id), sqlc.arg(received_at), sqlc.arg(expires_at))
ON CONFLICT(idempotency_key) DO UPDATE SET
  bridge_instance_id = excluded.bridge_instance_id,
  received_at = excluded.received_at, expires_at = excluded.expires_at;

-- name: GetBridgeIngestDedup :one
SELECT idempotency_key, bridge_instance_id, received_at, expires_at
FROM bridge_ingest_dedup
WHERE idempotency_key = sqlc.arg(idempotency_key) AND expires_at > sqlc.arg(lookup_at);

-- name: DeleteExpiredBridgeIngestDedup :execrows
DELETE FROM bridge_ingest_dedup WHERE expires_at <= sqlc.arg(now);

-- name: UpsertBridgeTaskSubscription :exec
INSERT INTO bridge_task_subscriptions (
  subscription_id, task_id, bridge_instance_id, scope, workspace_id,
  peer_id, thread_id, group_id, delivery_mode, created_by_kind,
  created_by_ref, created_at, updated_at
) VALUES (
  sqlc.arg(subscription_id), sqlc.arg(task_id), sqlc.arg(bridge_instance_id),
  sqlc.arg(scope), sqlc.narg(workspace_id), sqlc.narg(peer_id), sqlc.narg(thread_id),
  sqlc.narg(group_id), sqlc.arg(delivery_mode), sqlc.arg(created_by_kind),
  sqlc.arg(created_by_ref), sqlc.arg(created_at), sqlc.arg(updated_at)
)
ON CONFLICT(subscription_id) DO UPDATE SET
  task_id = excluded.task_id, bridge_instance_id = excluded.bridge_instance_id,
  scope = excluded.scope, workspace_id = excluded.workspace_id, peer_id = excluded.peer_id,
  thread_id = excluded.thread_id, group_id = excluded.group_id,
  delivery_mode = excluded.delivery_mode, updated_at = excluded.updated_at;

-- name: GetBridgeTaskSubscription :one
SELECT subscription_id, task_id, bridge_instance_id, scope, workspace_id,
       peer_id, thread_id, group_id, delivery_mode, created_by_kind,
       created_by_ref, created_at, updated_at
FROM bridge_task_subscriptions WHERE subscription_id = sqlc.arg(subscription_id);

-- name: DeleteBridgeTaskSubscription :execrows
DELETE FROM bridge_task_subscriptions WHERE subscription_id = sqlc.arg(subscription_id);
