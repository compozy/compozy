-- +goose Up
-- Remove trigger definitions that cannot be activated by any runtime producer.
-- Historical run rows remain as audit records. Catalog rows cascade with legacy
-- definitions; overlays, gateway bindings, and owned secrets are cleaned explicitly.
CREATE TEMP TABLE invalid_automation_trigger_events (
  trigger_id TEXT PRIMARY KEY
) WITHOUT ROWID;

WITH whitespace(chars) AS (
  VALUES (
    char(9) || char(10) || char(11) || char(12) || char(13) || char(32) ||
    char(133) || char(160) || char(5760) || char(8192) || char(8193) ||
    char(8194) || char(8195) || char(8196) || char(8197) || char(8198) ||
    char(8199) || char(8200) || char(8201) || char(8202) || char(8232) ||
    char(8233) || char(8239) || char(8287) || char(12288)
  )
), normalized AS (
  SELECT
    trigger.id,
    trigger.event,
    trim(trigger.event, whitespace.chars) AS canonical_event,
    substr(trigger.event, 6, length(trigger.event) - 15) AS hook_name,
    substr(trigger.event, 5) AS extension_suffix,
    whitespace.chars AS whitespace_chars
  FROM automation_triggers AS trigger
  CROSS JOIN whitespace
)
INSERT INTO invalid_automation_trigger_events (trigger_id)
SELECT id
FROM normalized
WHERE event != canonical_event
   OR NOT (
     event IN ('session.created', 'session.stopped', 'memory.consolidated', 'webhook')
     OR (
       substr(event, 1, 5) = 'hook.'
       AND substr(event, -10) = '.completed'
       AND length(event) > 15
       AND hook_name = trim(hook_name, whitespace_chars)
       AND length(hook_name) > 0
     )
     OR (
       substr(event, 1, 4) = 'ext.'
       AND length(trim(extension_suffix, whitespace_chars)) > 0
     )
   );

CREATE TEMP TABLE invalid_automation_trigger_resources (
  trigger_id TEXT PRIMARY KEY
) WITHOUT ROWID;

WITH whitespace(chars) AS (
  VALUES (
    char(9) || char(10) || char(11) || char(12) || char(13) || char(32) ||
    char(133) || char(160) || char(5760) || char(8192) || char(8193) ||
    char(8194) || char(8195) || char(8196) || char(8197) || char(8198) ||
    char(8199) || char(8200) || char(8201) || char(8202) || char(8232) ||
    char(8233) || char(8239) || char(8287) || char(12288)
  )
), normalized AS (
  SELECT
    resource.id,
    CASE WHEN json_valid(resource.spec_json)
      THEN CAST(json_extract(resource.spec_json, '$.event') AS TEXT)
    END AS event,
    CASE WHEN json_valid(resource.spec_json)
      THEN trim(CAST(json_extract(resource.spec_json, '$.event') AS TEXT), whitespace.chars)
    END AS canonical_event,
    CASE WHEN json_valid(resource.spec_json)
      THEN substr(CAST(json_extract(resource.spec_json, '$.event') AS TEXT), 6,
        length(CAST(json_extract(resource.spec_json, '$.event') AS TEXT)) - 15)
    END AS hook_name,
    CASE WHEN json_valid(resource.spec_json)
      THEN substr(CAST(json_extract(resource.spec_json, '$.event') AS TEXT), 5)
    END AS extension_suffix,
    whitespace.chars AS whitespace_chars
  FROM resource_records AS resource
  CROSS JOIN whitespace
  WHERE resource.kind = 'automation.trigger'
)
INSERT INTO invalid_automation_trigger_resources (trigger_id)
SELECT id
FROM normalized
WHERE event IS NULL
   OR event != canonical_event
   OR NOT (
     event IN ('session.created', 'session.stopped', 'memory.consolidated', 'webhook')
     OR (
       substr(event, 1, 5) = 'hook.'
       AND substr(event, -10) = '.completed'
       AND length(event) > 15
       AND hook_name = trim(hook_name, whitespace_chars)
       AND length(hook_name) > 0
     )
     OR (
       substr(event, 1, 4) = 'ext.'
       AND length(trim(extension_suffix, whitespace_chars)) > 0
     )
   );

CREATE TEMP TABLE invalid_automation_trigger_secret_refs (
  ref TEXT PRIMARY KEY
) WITHOUT ROWID;

INSERT INTO invalid_automation_trigger_secret_refs (ref)
SELECT trigger.webhook_secret_ref
FROM automation_triggers AS trigger
JOIN invalid_automation_trigger_events AS invalid ON invalid.trigger_id = trigger.id
WHERE trigger.webhook_secret_ref = 'vault:automation/triggers/' || trigger.id || '/webhook-secret'
UNION
SELECT CAST(json_extract(resource.spec_json, '$.webhook_secret_ref') AS TEXT)
FROM resource_records AS resource
JOIN invalid_automation_trigger_resources AS invalid ON invalid.trigger_id = resource.id
WHERE resource.kind = 'automation.trigger'
  AND json_valid(resource.spec_json)
  AND json_extract(resource.spec_json, '$.webhook_secret_ref') =
    'vault:automation/triggers/' || resource.id || '/webhook-secret';

DELETE FROM automation_triggers
WHERE id IN (SELECT trigger_id FROM invalid_automation_trigger_events);

DELETE FROM resource_records
WHERE kind = 'automation.trigger'
  AND id IN (SELECT trigger_id FROM invalid_automation_trigger_resources);

DELETE FROM automation_trigger_overlays
WHERE trigger_id IN (
    SELECT trigger_id FROM invalid_automation_trigger_events
    UNION
    SELECT trigger_id FROM invalid_automation_trigger_resources
  )
  AND NOT EXISTS (
    SELECT 1 FROM automation_triggers AS trigger
    WHERE trigger.id = automation_trigger_overlays.trigger_id
  )
  AND NOT EXISTS (
    SELECT 1 FROM resource_records AS resource
    WHERE resource.kind = 'automation.trigger'
      AND resource.id = automation_trigger_overlays.trigger_id
  );

DELETE FROM gateway_ingress_bindings
WHERE subject_kind = 'webhook_trigger'
  AND subject_id IN (
    SELECT trigger_id FROM invalid_automation_trigger_events
    UNION
    SELECT trigger_id FROM invalid_automation_trigger_resources
  )
  AND NOT EXISTS (
    SELECT 1 FROM automation_triggers AS trigger
    WHERE trigger.id = gateway_ingress_bindings.subject_id
  )
  AND NOT EXISTS (
    SELECT 1 FROM resource_records AS resource
    WHERE resource.kind = 'automation.trigger'
      AND resource.id = gateway_ingress_bindings.subject_id
  );

DELETE FROM vault_secrets
WHERE ref IN (SELECT ref FROM invalid_automation_trigger_secret_refs)
  AND NOT EXISTS (
    SELECT 1 FROM automation_triggers AS trigger
    WHERE trigger.webhook_secret_ref = vault_secrets.ref
  )
  AND NOT EXISTS (
    SELECT 1 FROM resource_records AS resource
    WHERE resource.kind = 'automation.trigger'
      AND json_valid(resource.spec_json)
      AND json_extract(resource.spec_json, '$.webhook_secret_ref') = vault_secrets.ref
  );

DROP TABLE invalid_automation_trigger_secret_refs;
DROP TABLE invalid_automation_trigger_events;
DROP TABLE invalid_automation_trigger_resources;
