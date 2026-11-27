-- name: GetWebhookIngressSubject :one
SELECT scope_kind, workspace_id, endpoint_slug, webhook_id, profile_id
FROM (
  SELECT
    resource.scope_kind,
    CASE resource.scope_kind
      WHEN 'profile' THEN (SELECT id FROM profiles WHERE name = resource.scope_id)
      WHEN 'workspace_profile' THEN (SELECT id FROM profiles WHERE name = substr(resource.scope_id, instr(resource.scope_id, '@pf:') + 4))
      ELSE '00000000000000000000000000'
    END AS profile_id,
    resource.scope_id AS workspace_id,
    CAST(json_extract(resource.spec_json, '$.endpoint_slug') AS TEXT) AS endpoint_slug,
    CAST(json_extract(resource.spec_json, '$.webhook_id') AS TEXT) AS webhook_id,
    0 AS source_order
  FROM resource_records AS resource
  WHERE resource.kind = 'automation.trigger' AND resource.id = sqlc.arg(subject_id)
  UNION ALL
  SELECT
    legacy.scope AS scope_kind,
    legacy.profile_id,
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

-- name: UpsertIngressBinding :one
INSERT INTO ingress_bindings (
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

-- name: DeleteIngressBinding :execrows
DELETE FROM ingress_bindings
WHERE subject_kind = sqlc.arg(subject_kind) AND subject_id = sqlc.arg(subject_id);

-- name: DeleteIngressBindingsByWorkspace :execrows
DELETE FROM ingress_bindings
WHERE scope_kind = 'workspace' AND workspace_id = sqlc.arg(workspace_id);

-- name: DeleteMismatchedIngressBinding :execrows
DELETE FROM ingress_bindings
WHERE subject_kind = sqlc.arg(subject_kind)
  AND subject_id = sqlc.arg(subject_id)
  AND (
    scope_kind <> sqlc.arg(scope_kind)
    OR COALESCE(workspace_id, '') <> COALESCE(sqlc.narg(workspace_id), '')
  );

-- name: GetIngressBinding :one
SELECT subject_kind, subject_id, scope_kind, workspace_id, endpoint_generation, confirmed_at
FROM ingress_bindings AS binding
WHERE binding.subject_kind = sqlc.arg(subject_kind)
  AND binding.subject_id = sqlc.arg(subject_id)
  AND (
    (binding.subject_kind = 'webhook_trigger' AND EXISTS (
      SELECT 1 FROM automation_triggers AS trigger WHERE trigger.id = binding.subject_id
    )) OR
    (binding.subject_kind = 'webhook_trigger' AND EXISTS (
      SELECT 1 FROM resource_records AS trigger
      WHERE trigger.kind = 'automation.trigger' AND trigger.id = binding.subject_id
    ))
)
ORDER BY subject_kind, subject_id;

-- name: SweepOrphanedIngressBindings :execrows
DELETE FROM ingress_bindings AS binding
WHERE (
  binding.subject_kind = 'webhook_trigger' AND NOT EXISTS (
    SELECT 1 FROM automation_triggers AS trigger WHERE trigger.id = binding.subject_id
  ) AND NOT EXISTS (
    SELECT 1 FROM resource_records AS trigger
    WHERE trigger.kind = 'automation.trigger' AND trigger.id = binding.subject_id
  )
);

-- name: ListIngressBindings :many
SELECT subject_kind, subject_id, scope_kind, workspace_id, endpoint_generation, confirmed_at
FROM ingress_bindings AS binding
WHERE (
  (binding.subject_kind = 'webhook_trigger' AND EXISTS (
    SELECT 1 FROM automation_triggers AS trigger WHERE trigger.id = binding.subject_id
  )) OR
  (binding.subject_kind = 'webhook_trigger' AND EXISTS (
    SELECT 1 FROM resource_records AS trigger
    WHERE trigger.kind = 'automation.trigger' AND trigger.id = binding.subject_id
  ))
)
ORDER BY subject_kind, subject_id;
