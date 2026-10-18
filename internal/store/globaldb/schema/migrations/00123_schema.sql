-- +goose Up
-- create index "idx_sessions_global_catalog_recent" to table: "sessions"
CREATE INDEX `idx_sessions_global_catalog_recent` ON `sessions` (`archived_at`, `updated_at` DESC, `created_at` DESC, `id` DESC);
-- create index "idx_sessions_global_catalog_activity" to table: "sessions"
CREATE INDEX idx_sessions_global_catalog_activity
 ON sessions(archived_at, COALESCE(last_update_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC);
-- create index "idx_sessions_global_catalog_attention" to table: "sessions"
CREATE INDEX idx_sessions_global_catalog_attention
 ON sessions(archived_at, (CASE
 WHEN (stop_verification_failed = 1 AND state <> 'stopped')
 OR pending_permission_count > 0 OR pending_clarify_count > 0
 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure')
 OR (state = 'stopped' AND trim(COALESCE(failure_kind, '')) <> '' AND trim(COALESCE(failure_kind, '')) <> 'cancellation') THEN 0
 WHEN state = 'active' AND trim(COALESCE(stall_state, '')) <> 'stalled'
 AND trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) = ''
 AND last_settled_revision > last_seen_revision THEN 1
 ELSE 2 END), COALESCE(attention_changed_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC);
