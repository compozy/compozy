-- name: UpsertGatewayEndpointState :one
INSERT INTO gateway_endpoint_state (
  tier, endpoint_url, endpoint_generation, reachable, updated_at
)
VALUES (sqlc.arg(tier), sqlc.arg(endpoint_url), 1, 1, sqlc.arg(updated_at))
ON CONFLICT(tier) DO UPDATE SET
  endpoint_url = excluded.endpoint_url,
  endpoint_generation = CASE
    WHEN gateway_endpoint_state.endpoint_url = excluded.endpoint_url
      THEN gateway_endpoint_state.endpoint_generation
    ELSE gateway_endpoint_state.endpoint_generation + 1
  END,
  reachable = 1,
  updated_at = excluded.updated_at
RETURNING tier, endpoint_url, endpoint_generation, reachable, updated_at;

-- name: MarkGatewayEndpointUnavailable :execrows
UPDATE gateway_endpoint_state
SET reachable = 0, updated_at = sqlc.arg(updated_at)
WHERE tier = sqlc.arg(tier) AND reachable <> 0;

-- name: GetGatewayEndpointState :one
SELECT tier, endpoint_url, endpoint_generation, reachable, updated_at
FROM gateway_endpoint_state
WHERE tier = sqlc.arg(tier);

-- name: ListGatewayEndpointStates :many
SELECT tier, endpoint_url, endpoint_generation, reachable, updated_at
FROM gateway_endpoint_state
ORDER BY tier;

-- name: GetGatewayWebhookIngressSubject :one
SELECT scope_kind, workspace_id, endpoint_slug, webhook_id
FROM (
  SELECT
    resource.scope_kind,
    resource.scope_id AS workspace_id,
    CAST(json_extract(resource.spec_json, '$.endpoint_slug') AS TEXT) AS endpoint_slug,
    CAST(json_extract(resource.spec_json, '$.webhook_id') AS TEXT) AS webhook_id,
    0 AS source_order
  FROM resource_records AS resource
  WHERE resource.kind = 'automation.trigger' AND resource.id = sqlc.arg(subject_id)
  UNION ALL
  SELECT
    legacy.scope AS scope_kind,
    legacy.workspace_id,
    legacy.endpoint_slug,
    legacy.webhook_id,
    1 AS source_order
  FROM automation_triggers AS legacy
  WHERE legacy.id = sqlc.arg(subject_id)
)
WHERE webhook_id IS NOT NULL
  AND length(trim(webhook_id)) > 0
  AND endpoint_slug IS NOT NULL
  AND length(trim(endpoint_slug)) > 0
ORDER BY source_order
LIMIT 1;

-- name: GetGatewayBridgeIngressSubject :one
SELECT scope_kind, workspace_id, provider_config
FROM (
  SELECT
    resource.scope_kind,
    resource.scope_id AS workspace_id,
    CAST(json_extract(resource.spec_json, '$.provider_config') AS TEXT) AS provider_config,
    0 AS source_order
  FROM resource_records AS resource
  WHERE resource.kind = 'bridge.instance' AND resource.id = sqlc.arg(subject_id)
  UNION ALL
  SELECT
    legacy.scope AS scope_kind,
    legacy.workspace_id,
    legacy.provider_config,
    1 AS source_order
  FROM bridge_instances AS legacy
  WHERE legacy.id = sqlc.arg(subject_id)
)
ORDER BY source_order
LIMIT 1;

-- name: UpsertGatewayIngressBinding :one
INSERT INTO gateway_ingress_bindings (
  subject_kind, subject_id, scope_kind, workspace_id, endpoint_generation, confirmed_at
)
VALUES (
  sqlc.arg(subject_kind), sqlc.arg(subject_id), sqlc.arg(scope_kind),
  sqlc.narg(workspace_id), sqlc.arg(endpoint_generation), sqlc.arg(confirmed_at)
)
ON CONFLICT(subject_kind, subject_id) DO UPDATE SET
  scope_kind = excluded.scope_kind,
  workspace_id = excluded.workspace_id,
  endpoint_generation = excluded.endpoint_generation,
  confirmed_at = excluded.confirmed_at
RETURNING subject_kind, subject_id, scope_kind, workspace_id, endpoint_generation, confirmed_at;

-- name: DeleteGatewayIngressBinding :execrows
DELETE FROM gateway_ingress_bindings
WHERE subject_kind = sqlc.arg(subject_kind) AND subject_id = sqlc.arg(subject_id);

-- name: DeleteGatewayIngressBindingsByWorkspace :execrows
DELETE FROM gateway_ingress_bindings
WHERE scope_kind = 'workspace' AND workspace_id = sqlc.arg(workspace_id);

-- name: DeleteMismatchedGatewayIngressBinding :execrows
DELETE FROM gateway_ingress_bindings
WHERE subject_kind = sqlc.arg(subject_kind)
  AND subject_id = sqlc.arg(subject_id)
  AND (
    scope_kind <> sqlc.arg(scope_kind)
    OR COALESCE(workspace_id, '') <> COALESCE(sqlc.narg(workspace_id), '')
  );

-- name: GetGatewayIngressBinding :one
SELECT subject_kind, subject_id, scope_kind, workspace_id, endpoint_generation, confirmed_at
FROM gateway_ingress_bindings AS binding
WHERE binding.subject_kind = sqlc.arg(subject_kind)
  AND binding.subject_id = sqlc.arg(subject_id)
  AND (
    (binding.subject_kind = 'webhook_trigger' AND EXISTS (
      SELECT 1 FROM automation_triggers AS trigger WHERE trigger.id = binding.subject_id
    )) OR
    (binding.subject_kind = 'webhook_trigger' AND EXISTS (
      SELECT 1 FROM resource_records AS trigger
      WHERE trigger.kind = 'automation.trigger' AND trigger.id = binding.subject_id
    )) OR
    (binding.subject_kind = 'bridge_instance' AND EXISTS (
      SELECT 1 FROM bridge_instances AS bridge WHERE bridge.id = binding.subject_id
    )) OR
    (binding.subject_kind = 'bridge_instance' AND EXISTS (
      SELECT 1 FROM resource_records AS bridge
      WHERE bridge.kind = 'bridge.instance' AND bridge.id = binding.subject_id
    ))
  );

-- name: ListGatewayIngressBindings :many
SELECT subject_kind, subject_id, scope_kind, workspace_id, endpoint_generation, confirmed_at
FROM gateway_ingress_bindings AS binding
WHERE (
  (binding.subject_kind = 'webhook_trigger' AND EXISTS (
    SELECT 1 FROM automation_triggers AS trigger WHERE trigger.id = binding.subject_id
  )) OR
  (binding.subject_kind = 'webhook_trigger' AND EXISTS (
    SELECT 1 FROM resource_records AS trigger
    WHERE trigger.kind = 'automation.trigger' AND trigger.id = binding.subject_id
  )) OR
  (binding.subject_kind = 'bridge_instance' AND EXISTS (
    SELECT 1 FROM bridge_instances AS bridge WHERE bridge.id = binding.subject_id
  )) OR
  (binding.subject_kind = 'bridge_instance' AND EXISTS (
    SELECT 1 FROM resource_records AS bridge
    WHERE bridge.kind = 'bridge.instance' AND bridge.id = binding.subject_id
  ))
)
ORDER BY subject_kind, subject_id;

-- name: SweepOrphanedGatewayIngressBindings :execrows
DELETE FROM gateway_ingress_bindings AS binding
WHERE (
  binding.subject_kind = 'webhook_trigger' AND NOT EXISTS (
    SELECT 1 FROM automation_triggers AS trigger WHERE trigger.id = binding.subject_id
  ) AND NOT EXISTS (
    SELECT 1 FROM resource_records AS trigger
    WHERE trigger.kind = 'automation.trigger' AND trigger.id = binding.subject_id
  )
) OR (
  binding.subject_kind = 'bridge_instance' AND NOT EXISTS (
    SELECT 1 FROM bridge_instances AS bridge WHERE bridge.id = binding.subject_id
  ) AND NOT EXISTS (
    SELECT 1 FROM resource_records AS bridge
    WHERE bridge.kind = 'bridge.instance' AND bridge.id = binding.subject_id
  )
);
