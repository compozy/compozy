-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_memory_catalog_entries" table
CREATE TABLE `new_memory_catalog_entries` (`id` text NULL, `workspace_id` text NOT NULL DEFAULT '', `profile_id` text NOT NULL DEFAULT '', `scope` text NOT NULL, `agent_name` text NOT NULL DEFAULT '', `agent_tier` text NOT NULL DEFAULT '', `type` text NOT NULL, `slug` text NOT NULL, `filename` text NOT NULL, `name` text NOT NULL DEFAULT '', `description` text NOT NULL DEFAULT '', `content` text NOT NULL DEFAULT '', `content_hash` text NOT NULL, `injection` integer NOT NULL DEFAULT 1, `mtime_ms` integer NOT NULL, `indexed_at` integer NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CHECK (scope IN ('profile', 'workspace', 'agent')), CHECK (agent_tier IN ('', 'workspace', 'global')), CHECK (type IN ('user', 'feedback', 'project', 'reference')));
-- copy rows from old table "memory_catalog_entries" to new temporary table "new_memory_catalog_entries"
INSERT INTO `new_memory_catalog_entries` (`id`, `workspace_id`, `profile_id`, `scope`, `agent_name`, `agent_tier`, `type`, `slug`, `filename`, `name`, `description`, `content`, `content_hash`, `injection`, `mtime_ms`, `indexed_at`, `updated_at`) SELECT `id`, `workspace_id`, CASE WHEN `scope` = 'global' THEN '00000000000000000000000000' ELSE '' END, CASE WHEN `scope` = 'global' THEN 'profile' ELSE `scope` END, `agent_name`, `agent_tier`, `type`, `slug`, `filename`, `name`, `description`, `content`, `content_hash`, `injection`, `mtime_ms`, `indexed_at`, `updated_at` FROM `memory_catalog_entries`;
-- drop trigger "memory_catalog_entries_ad" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `memory_catalog_entries_ad`;
-- drop trigger "memory_catalog_entries_ai" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `memory_catalog_entries_ai`;
-- drop trigger "memory_catalog_entries_au" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `memory_catalog_entries_au`;
-- drop "memory_catalog_entries" table after copying rows
DROP TABLE `memory_catalog_entries`;
-- rename temporary table "new_memory_catalog_entries" to "memory_catalog_entries"
ALTER TABLE `new_memory_catalog_entries` RENAME TO `memory_catalog_entries`;
-- create index "idx_memory_catalog_scope" to table: "memory_catalog_entries"
CREATE INDEX `idx_memory_catalog_scope` ON `memory_catalog_entries` (`profile_id`, `scope`, `agent_name`, `agent_tier`, `type`);
-- create index "idx_memory_catalog_updated_at" to table: "memory_catalog_entries"
CREATE INDEX `idx_memory_catalog_updated_at` ON `memory_catalog_entries` (`updated_at`);
-- create index "idx_memory_catalog_workspace" to table: "memory_catalog_entries"
CREATE INDEX `idx_memory_catalog_workspace` ON `memory_catalog_entries` (`workspace_id`, `profile_id`);
-- create index "uq_memory_catalog_new_scope_slug" to table: "memory_catalog_entries"
CREATE UNIQUE INDEX `uq_memory_catalog_new_scope_slug` ON `memory_catalog_entries` (`workspace_id`, `profile_id`, `scope`, `agent_name`, `agent_tier`, `type`, `slug`);
-- create index "uq_memory_catalog_scope_slug" to table: "memory_catalog_entries"
CREATE UNIQUE INDEX `uq_memory_catalog_scope_slug` ON `memory_catalog_entries` (`workspace_id`, `profile_id`, `scope`, `agent_name`, `agent_tier`, `type`, `slug`);
-- rebuild the external-content FTS table so profile ownership is part of its durable identity
DROP TABLE `memory_catalog_fts`;
CREATE VIRTUAL TABLE memory_catalog_fts USING fts5(
	name,
	description,
	content,
	profile_id UNINDEXED,
	content='memory_catalog_entries',
	content_rowid='rowid',
	tokenize='porter unicode61'
);
-- create "new_memory_consolidations" table
CREATE TABLE `new_memory_consolidations` (`id` text NULL, `workspace_id` text NULL, `profile_id` text NOT NULL DEFAULT '', `scope` text NOT NULL, `agent_name` text NULL, `agent_tier` text NULL, `started_at` integer NOT NULL, `finished_at` integer NULL, `status` text NOT NULL, `input_count` integer NOT NULL DEFAULT 0, `promoted_count` integer NOT NULL DEFAULT 0, `error` text NOT NULL DEFAULT '', `metadata` text NOT NULL DEFAULT '{}', PRIMARY KEY (`id`), CHECK (scope IN ('profile', 'workspace', 'agent')), CHECK (agent_tier IS NULL OR agent_tier IN ('workspace', 'global')), CHECK (status IN ('running', 'completed', 'failed', 'canceled')));
-- copy rows from old table "memory_consolidations" to new temporary table "new_memory_consolidations"
INSERT INTO `new_memory_consolidations` (`id`, `workspace_id`, `profile_id`, `scope`, `agent_name`, `agent_tier`, `started_at`, `finished_at`, `status`, `input_count`, `promoted_count`, `error`, `metadata`) SELECT `id`, `workspace_id`, CASE WHEN `scope` = 'global' THEN '00000000000000000000000000' ELSE '' END, CASE WHEN `scope` = 'global' THEN 'profile' ELSE `scope` END, `agent_name`, `agent_tier`, `started_at`, `finished_at`, `status`, `input_count`, `promoted_count`, `error`, `metadata` FROM `memory_consolidations`;
-- drop "memory_consolidations" table after copying rows
DROP TABLE `memory_consolidations`;
-- rename temporary table "new_memory_consolidations" to "memory_consolidations"
ALTER TABLE `new_memory_consolidations` RENAME TO `memory_consolidations`;
-- create index "idx_consolidations_status" to table: "memory_consolidations"
CREATE INDEX `idx_consolidations_status` ON `memory_consolidations` (`status`, `started_at`);
-- create index "idx_consolidations_workspace" to table: "memory_consolidations"
CREATE INDEX `idx_consolidations_workspace` ON `memory_consolidations` (`workspace_id`, `profile_id`, `started_at`);
-- create "new_memory_decisions" table
CREATE TABLE `new_memory_decisions` (`id` text NULL, `candidate_hash` text NOT NULL, `idempotency_key` text NOT NULL, `frontmatter_hash` text NOT NULL, `workspace_id` text NULL, `profile_id` text NOT NULL DEFAULT '', `scope` text NOT NULL, `agent_name` text NULL, `agent_tier` text NULL, `op` text NOT NULL, `targets` text NOT NULL DEFAULT '[]', `target_filename` text NOT NULL, `frontmatter` text NOT NULL DEFAULT '{}', `post_content` text NULL, `post_content_hash` text NULL, `prior_content` text NULL, `confidence` real NOT NULL, `source` text NOT NULL, `rule_trace` text NOT NULL, `llm_trace` text NULL, `reason` text NULL, `prompt_version` text NULL, `applied_at` integer NULL, `decided_at` integer NOT NULL, PRIMARY KEY (`id`), CHECK (scope IN ('profile', 'workspace', 'agent')), CHECK (agent_tier IS NULL OR agent_tier IN ('workspace', 'global')), CHECK (op IN ('noop', 'add', 'update', 'delete', 'reject')), CHECK (source IN ('rule', 'llm')));
-- copy rows from old table "memory_decisions" to new temporary table "new_memory_decisions"
INSERT INTO `new_memory_decisions` (`id`, `candidate_hash`, `idempotency_key`, `frontmatter_hash`, `workspace_id`, `profile_id`, `scope`, `agent_name`, `agent_tier`, `op`, `targets`, `target_filename`, `frontmatter`, `post_content`, `post_content_hash`, `prior_content`, `confidence`, `source`, `rule_trace`, `llm_trace`, `reason`, `prompt_version`, `applied_at`, `decided_at`) SELECT `id`, `candidate_hash`, `idempotency_key`, `frontmatter_hash`, `workspace_id`, CASE WHEN `scope` = 'global' THEN '00000000000000000000000000' ELSE '' END, CASE WHEN `scope` = 'global' THEN 'profile' ELSE `scope` END, `agent_name`, `agent_tier`, `op`, `targets`, `target_filename`, `frontmatter`, `post_content`, `post_content_hash`, `prior_content`, `confidence`, `source`, `rule_trace`, `llm_trace`, `reason`, `prompt_version`, `applied_at`, `decided_at` FROM `memory_decisions`;
-- drop "memory_decisions" table after copying rows
DROP TABLE `memory_decisions`;
-- rename temporary table "new_memory_decisions" to "memory_decisions"
ALTER TABLE `new_memory_decisions` RENAME TO `memory_decisions`;
-- create index "memory_decisions_idempotency_key" to table: "memory_decisions"
CREATE UNIQUE INDEX `memory_decisions_idempotency_key` ON `memory_decisions` (`idempotency_key`);
-- create index "idx_decisions_op" to table: "memory_decisions"
CREATE INDEX `idx_decisions_op` ON `memory_decisions` (`op`, `decided_at`);
-- create index "idx_decisions_unapplied" to table: "memory_decisions"
CREATE INDEX `idx_decisions_unapplied` ON `memory_decisions` (`applied_at`) WHERE applied_at IS NULL;
-- create index "idx_decisions_workspace" to table: "memory_decisions"
CREATE INDEX `idx_decisions_workspace` ON `memory_decisions` (`workspace_id`, `profile_id`, `decided_at`);
-- create "new_memory_events" table
CREATE TABLE `new_memory_events` (`id` integer NULL PRIMARY KEY AUTOINCREMENT, `op` text NOT NULL, `profile_id` text NOT NULL DEFAULT '', `scope` text NULL, `agent_name` text NULL, `agent_tier` text NULL, `workspace_id` text NULL, `session_id` text NULL, `actor_kind` text NOT NULL, `decision_id` text NULL, `target_id` text NULL, `metadata` text NOT NULL DEFAULT '{}', `ts_ms` integer NOT NULL, CHECK (op IN ('memory.write.committed', 'memory.write.rejected', 'memory.write.shadowed', 'memory.write.reindex', 'memory.write.reverted', 'memory.recall.executed', 'memory.recall.skipped', 'memory.recall.signal_dropped', 'memory.recall.signal_update_failed', 'memory.decisions.audit_summarized', 'memory.decisions.pruned', 'memory.dream.run.started', 'memory.dream.run.promoted', 'memory.dream.run.failed', 'memory.extractor.started', 'memory.extractor.completed', 'memory.extractor.failed', 'memory.extractor.coalesced', 'memory.extractor.dropped', 'memory.daily.rotated', 'memory.daily.archived', 'memory.daily.restored', 'memory.daily.purged', 'memory.daily.archive_purged', 'memory.provider.enabled', 'memory.provider.disabled', 'memory.provider.collision', 'memory.workspace.relocated', 'memory.workspace.recovered', 'memory.agent.purged', 'memory.migration.applied')), CHECK (scope IN ('profile', 'workspace', 'agent')), CHECK (agent_tier IS NULL OR agent_tier IN ('workspace', 'global')));
-- copy rows from old table "memory_events" to new temporary table "new_memory_events"
INSERT INTO `new_memory_events` (`id`, `op`, `profile_id`, `scope`, `agent_name`, `agent_tier`, `workspace_id`, `session_id`, `actor_kind`, `decision_id`, `target_id`, `metadata`, `ts_ms`) SELECT `id`, `op`, CASE WHEN `scope` = 'global' THEN '00000000000000000000000000' ELSE '' END, CASE WHEN `scope` = 'global' THEN 'profile' ELSE `scope` END, `agent_name`, `agent_tier`, `workspace_id`, `session_id`, `actor_kind`, `decision_id`, `target_id`, `metadata`, `ts_ms` FROM `memory_events`;
-- drop "memory_events" table after copying rows
DROP TABLE `memory_events`;
-- rename temporary table "new_memory_events" to "memory_events"
ALTER TABLE `new_memory_events` RENAME TO `memory_events`;
-- create index "idx_events_op" to table: "memory_events"
CREATE INDEX `idx_events_op` ON `memory_events` (`op`, `ts_ms`);
-- create index "idx_events_session" to table: "memory_events"
CREATE INDEX `idx_events_session` ON `memory_events` (`session_id`, `ts_ms`);
-- create index "idx_events_workspace" to table: "memory_events"
CREATE INDEX `idx_events_workspace` ON `memory_events` (`workspace_id`, `profile_id`, `ts_ms`);
-- create "new_memory_recall_signals" table
CREATE TABLE `new_memory_recall_signals` (`chunk_id` text NULL, `workspace_id` text NULL, `profile_id` text NOT NULL DEFAULT '', `recall_count` integer NOT NULL DEFAULT 0, `last_recalled_at` integer NULL, `recall_score` real NOT NULL DEFAULT 0, `freshness_started_at` integer NOT NULL DEFAULT 0, `promoted_at` integer NULL, `promotion_run_id` text NULL, `last_score_update_at` integer NOT NULL DEFAULT 0, `session_count` integer NOT NULL DEFAULT 0, `last_session_id` text NULL, `already_surfaced_json` text NOT NULL DEFAULT '[]', `updated_at` integer NOT NULL, PRIMARY KEY (`chunk_id`), CONSTRAINT `0` FOREIGN KEY (`chunk_id`) REFERENCES `memory_chunks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE);
-- copy rows from old table "memory_recall_signals" to new temporary table "new_memory_recall_signals"
INSERT INTO `new_memory_recall_signals` (`chunk_id`, `workspace_id`, `profile_id`, `recall_count`, `last_recalled_at`, `recall_score`, `freshness_started_at`, `promoted_at`, `promotion_run_id`, `last_score_update_at`, `session_count`, `last_session_id`, `already_surfaced_json`, `updated_at`) SELECT sig.`chunk_id`, sig.`workspace_id`, CASE WHEN entry.`scope` = 'profile' THEN entry.`profile_id` ELSE '' END, sig.`recall_count`, sig.`last_recalled_at`, sig.`recall_score`, sig.`freshness_started_at`, sig.`promoted_at`, sig.`promotion_run_id`, sig.`last_score_update_at`, sig.`session_count`, sig.`last_session_id`, sig.`already_surfaced_json`, sig.`updated_at` FROM `memory_recall_signals` sig JOIN `memory_chunks` chunk ON chunk.`id` = sig.`chunk_id` JOIN `memory_catalog_entries` entry ON entry.`id` = chunk.`file_id`;
-- drop "memory_recall_signals" table after copying rows
DROP TABLE `memory_recall_signals`;
-- rename temporary table "new_memory_recall_signals" to "memory_recall_signals"
ALTER TABLE `new_memory_recall_signals` RENAME TO `memory_recall_signals`;
-- create index "idx_recall_signals_last_recalled" to table: "memory_recall_signals"
CREATE INDEX `idx_recall_signals_last_recalled` ON `memory_recall_signals` (`last_recalled_at`);
-- create index "idx_recall_signals_workspace" to table: "memory_recall_signals"
CREATE INDEX `idx_recall_signals_workspace` ON `memory_recall_signals` (`workspace_id`, `profile_id`, `updated_at`);
-- create index "idx_signals_recent" to table: "memory_recall_signals"
CREATE INDEX `idx_signals_recent` ON `memory_recall_signals` (`last_recalled_at`);
-- create index "idx_signals_unpromoted" to table: "memory_recall_signals"
CREATE INDEX `idx_signals_unpromoted` ON `memory_recall_signals` (`promoted_at`, `recall_score`) WHERE promoted_at IS NULL;
-- create "memory_maintenance_ops" table
CREATE TABLE `memory_maintenance_ops` (`op` text NULL, `status` text NOT NULL, `created_at` text NOT NULL, `completed_at` text NULL, PRIMARY KEY (`op`), CHECK (status IN ('pending', 'done')));
INSERT INTO memory_maintenance_ops (op, status, created_at)
VALUES ('move_global_dir', 'pending', strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- recreate trigger "memory_catalog_entries_ad" after rebuilding table "memory_catalog_entries"
-- +goose StatementBegin
CREATE TRIGGER memory_catalog_entries_ad AFTER DELETE ON memory_catalog_entries BEGIN
		INSERT INTO memory_catalog_fts(memory_catalog_fts, rowid, name, description, content, profile_id)
		VALUES ('delete', old.rowid, old.name, old.description, old.content, old.profile_id);
	END;
-- +goose StatementEnd
-- recreate trigger "memory_catalog_entries_ai" after rebuilding table "memory_catalog_entries"
-- +goose StatementBegin
CREATE TRIGGER memory_catalog_entries_ai AFTER INSERT ON memory_catalog_entries BEGIN
		INSERT INTO memory_catalog_fts(rowid, name, description, content, profile_id)
		VALUES (new.rowid, new.name, new.description, new.content, new.profile_id);
	END;
-- +goose StatementEnd
-- recreate trigger "memory_catalog_entries_au" after rebuilding table "memory_catalog_entries"
-- +goose StatementBegin
CREATE TRIGGER memory_catalog_entries_au AFTER UPDATE ON memory_catalog_entries BEGIN
		INSERT INTO memory_catalog_fts(memory_catalog_fts, rowid, name, description, content, profile_id)
		VALUES ('delete', old.rowid, old.name, old.description, old.content, old.profile_id);
		INSERT INTO memory_catalog_fts(rowid, name, description, content, profile_id)
		VALUES (new.rowid, new.name, new.description, new.content, new.profile_id);
	END;
-- +goose StatementEnd
INSERT INTO memory_catalog_fts(memory_catalog_fts) VALUES ('rebuild');
