-- +goose Up
-- create index "idx_sessions_global_catalog_navigator" to table: "sessions"
CREATE INDEX idx_sessions_global_catalog_navigator
 ON sessions(archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);
-- create index "idx_sessions_profile_global_catalog_navigator" to table: "sessions"
CREATE INDEX idx_sessions_profile_global_catalog_navigator
 ON sessions(profile_id, archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);
-- create index "idx_sessions_workspace_catalog_navigator" to table: "sessions"
CREATE INDEX idx_sessions_workspace_catalog_navigator
 ON sessions(workspace_id, archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);
-- create index "idx_sessions_profile_workspace_catalog_navigator" to table: "sessions"
CREATE INDEX idx_sessions_profile_workspace_catalog_navigator
 ON sessions(profile_id, workspace_id, archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);
-- create index "idx_sessions_catalog_created" to table: "sessions"
CREATE INDEX `idx_sessions_catalog_created` ON `sessions` (`workspace_id`, `archived_at`, `created_at` DESC, `id` DESC);
-- create index "idx_sessions_profile_catalog_created" to table: "sessions"
CREATE INDEX `idx_sessions_profile_catalog_created` ON `sessions` (`profile_id`, `workspace_id`, `archived_at`, `created_at` DESC, `id` DESC);
