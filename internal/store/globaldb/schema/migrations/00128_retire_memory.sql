-- +goose Up
-- Retire derived memory state; authored Markdown files are not touched.
PRAGMA foreign_keys = OFF;

DROP TRIGGER IF EXISTS memory_catalog_entries_ad;
DROP TRIGGER IF EXISTS memory_catalog_entries_ai;
DROP TRIGGER IF EXISTS memory_catalog_entries_au;
DROP TRIGGER IF EXISTS memory_chunks_ad;
DROP TRIGGER IF EXISTS memory_chunks_ai;
DROP TRIGGER IF EXISTS memory_chunks_au;
DROP INDEX IF EXISTS memory_decisions_idempotency_key;
DROP INDEX IF EXISTS idx_chunks_file;
DROP INDEX IF EXISTS idx_consolidations_status;
DROP INDEX IF EXISTS idx_consolidations_workspace;
DROP INDEX IF EXISTS idx_decisions_op;
DROP INDEX IF EXISTS idx_decisions_unapplied;
DROP INDEX IF EXISTS idx_decisions_workspace;
DROP INDEX IF EXISTS idx_events_op;
DROP INDEX IF EXISTS idx_events_session;
DROP INDEX IF EXISTS idx_events_workspace;
DROP INDEX IF EXISTS idx_memory_catalog_scope;
DROP INDEX IF EXISTS idx_memory_catalog_updated_at;
DROP INDEX IF EXISTS idx_memory_catalog_workspace;
DROP INDEX IF EXISTS idx_recall_signals_last_recalled;
DROP INDEX IF EXISTS idx_recall_signals_workspace;
DROP INDEX IF EXISTS idx_signals_recent;
DROP INDEX IF EXISTS idx_signals_unpromoted;
DROP INDEX IF EXISTS uq_memory_catalog_new_scope_slug;
DROP INDEX IF EXISTS uq_memory_catalog_scope_slug;

DROP TABLE IF EXISTS memory_recall_signals;
DROP TABLE IF EXISTS memory_chunks_fts;
DROP TABLE IF EXISTS memory_chunks_fts_trigram;
DROP TABLE IF EXISTS memory_chunks;
DROP TABLE IF EXISTS memory_catalog_fts;
DROP TABLE IF EXISTS memory_catalog_entries;
DROP TABLE IF EXISTS memory_catalog_state;
DROP TABLE IF EXISTS memory_consolidations;
DROP TABLE IF EXISTS memory_decisions;
DROP TABLE IF EXISTS memory_events;
DROP TABLE IF EXISTS memory_maintenance_ops;
DROP TABLE IF EXISTS goose_db_version_memory;

-- Capture ownership before deleting definitions; preserve automation run history.
CREATE TEMP TABLE retired_memory_trigger_events (trigger_id TEXT PRIMARY KEY) WITHOUT ROWID;
INSERT INTO retired_memory_trigger_events SELECT id FROM automation_triggers WHERE event = 'memory.consolidated';
CREATE TEMP TABLE retired_memory_trigger_resources (trigger_id TEXT PRIMARY KEY) WITHOUT ROWID;
INSERT INTO retired_memory_trigger_resources
SELECT id FROM resource_records
WHERE kind = 'automation.trigger' AND json_valid(spec_json)
  AND json_extract(spec_json, '$.event') = 'memory.consolidated';
CREATE TEMP TABLE retired_memory_trigger_secret_refs (
  ref TEXT PRIMARY KEY
) WITHOUT ROWID;

INSERT INTO retired_memory_trigger_secret_refs (ref)
SELECT trigger.webhook_secret_ref
FROM automation_triggers AS trigger
JOIN retired_memory_trigger_events AS invalid ON invalid.trigger_id = trigger.id
WHERE trigger.webhook_secret_ref = 'vault:automation/triggers/' || trigger.id || '/webhook-secret'
UNION
SELECT CAST(json_extract(resource.spec_json, '$.webhook_secret_ref') AS TEXT)
FROM resource_records AS resource
JOIN retired_memory_trigger_resources AS invalid ON invalid.trigger_id = resource.id
WHERE resource.kind = 'automation.trigger'
  AND json_valid(resource.spec_json)
  AND json_extract(resource.spec_json, '$.webhook_secret_ref') =
    'vault:automation/triggers/' || resource.id || '/webhook-secret';

DELETE FROM automation_trigger_catalog_filter_terms
WHERE trigger_id IN (SELECT trigger_id FROM retired_memory_trigger_events);
DELETE FROM automation_trigger_catalog_entries
WHERE trigger_id IN (SELECT trigger_id FROM retired_memory_trigger_events);

DELETE FROM automation_triggers
WHERE id IN (SELECT trigger_id FROM retired_memory_trigger_events);

DELETE FROM resource_records
WHERE kind = 'automation.trigger'
  AND id IN (SELECT trigger_id FROM retired_memory_trigger_resources);

DELETE FROM automation_trigger_overlays
WHERE trigger_id IN (
    SELECT trigger_id FROM retired_memory_trigger_events
    UNION
    SELECT trigger_id FROM retired_memory_trigger_resources
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
    SELECT trigger_id FROM retired_memory_trigger_events
    UNION
    SELECT trigger_id FROM retired_memory_trigger_resources
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
WHERE ref IN (SELECT ref FROM retired_memory_trigger_secret_refs)
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

DROP TABLE retired_memory_trigger_secret_refs;
DROP TABLE retired_memory_trigger_events;
DROP TABLE retired_memory_trigger_resources;

-- All nine REFERENCES sessions sites: delete CASCADE dependents in dependency order;
-- preserve heartbeat audit events with the same SET NULL semantics as their FK.
CREATE TEMP TABLE retired_memory_sessions (id TEXT PRIMARY KEY) WITHOUT ROWID;
INSERT INTO retired_memory_sessions
SELECT id FROM sessions WHERE session_type = 'dream' OR spawn_role = 'memory-extractor';
DELETE FROM session_input_clear_traces WHERE session_id IN (SELECT id FROM retired_memory_sessions);
DELETE FROM session_input_queue WHERE session_id IN (SELECT id FROM retired_memory_sessions);
DELETE FROM session_prompt_admissions WHERE session_id IN (SELECT id FROM retired_memory_sessions);
DELETE FROM session_health WHERE session_id IN (SELECT id FROM retired_memory_sessions);
DELETE FROM session_pending_interactions WHERE session_id IN (SELECT id FROM retired_memory_sessions);
DELETE FROM token_stats WHERE session_id IN (SELECT id FROM retired_memory_sessions);
DELETE FROM permission_log WHERE session_id IN (SELECT id FROM retired_memory_sessions);
DELETE FROM agent_heartbeat_wake_state WHERE session_id IN (SELECT id FROM retired_memory_sessions);
UPDATE agent_heartbeat_wake_events SET session_id = NULL
WHERE session_id IN (SELECT id FROM retired_memory_sessions);
DELETE FROM sessions WHERE id IN (SELECT id FROM retired_memory_sessions);
DROP TABLE retired_memory_sessions;
PRAGMA foreign_keys = ON;
