-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_agent_skill_reviews" table
CREATE TABLE `new_agent_skill_reviews` (`id` text NULL, `instance_id` text NOT NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `agent_name` text NOT NULL DEFAULT '', `skill_name` text NOT NULL, `digest` text NOT NULL, `state` text NOT NULL, `draft_path` text NOT NULL, `source_session_id` text NULL, `decided_by` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `decided_at` text NULL, `kind` text NOT NULL DEFAULT 'new', `skill_key` text NOT NULL DEFAULT '', `base_digest` text NOT NULL DEFAULT '', `snapshot_path` text NOT NULL DEFAULT '', `proposed_digest` text NOT NULL DEFAULT '', `installed_digest` text NOT NULL DEFAULT '', `prev_install_marker` blob NULL, `lesson` text NOT NULL DEFAULT '{}', `evidence_session_ids` text NOT NULL DEFAULT '[]', `decided_via` text NOT NULL DEFAULT '', `reverted_by` text NOT NULL DEFAULT '', `reverted_at` text NULL, PRIMARY KEY (`id`), CHECK (state IN ('pending', 'approved', 'rejected', 'reverted')), CHECK (kind IN ('new', 'edit')), CHECK (json_valid(lesson)), CHECK (json_valid(evidence_session_ids)), CHECK (decided_via IN ('', 'operator', 'auto_approve')));
-- copy every review; decisions recorded before this migration were made by an operator
INSERT INTO `new_agent_skill_reviews` (`id`, `instance_id`, `profile_id`, `workspace_id`, `agent_name`, `skill_name`, `digest`, `state`, `draft_path`, `source_session_id`, `decided_by`, `created_at`, `decided_at`, `decided_via`) SELECT `id`, `instance_id`, `profile_id`, `workspace_id`, `agent_name`, `skill_name`, `digest`, `state`, `draft_path`, `source_session_id`, `decided_by`, `created_at`, `decided_at`, CASE WHEN `state` IN ('approved', 'rejected') THEN 'operator' ELSE '' END FROM `agent_skill_reviews`;
-- drop "agent_skill_reviews" table after copying rows
DROP TABLE `agent_skill_reviews`;
-- rename temporary table "new_agent_skill_reviews" to "agent_skill_reviews"
ALTER TABLE `new_agent_skill_reviews` RENAME TO `agent_skill_reviews`;
-- create index "idx_agent_skill_reviews_instance" to table: "agent_skill_reviews"
CREATE INDEX `idx_agent_skill_reviews_instance` ON `agent_skill_reviews` (`instance_id`, `state`, `created_at`, `id`);
-- create index "idx_agent_skill_reviews_scope" to table: "agent_skill_reviews"
CREATE INDEX `idx_agent_skill_reviews_scope` ON `agent_skill_reviews` (`profile_id`, `workspace_id`, `created_at`, `id`);
-- create index "idx_agent_skill_reviews_key" to table: "agent_skill_reviews"
CREATE INDEX `idx_agent_skill_reviews_key` ON `agent_skill_reviews` (`skill_key`, `state`);
-- create index "idx_agent_skill_reviews_identity" to table: "agent_skill_reviews"
CREATE INDEX `idx_agent_skill_reviews_identity` ON `agent_skill_reviews` (`instance_id`, `skill_name`, `digest`, `base_digest`, `state`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
