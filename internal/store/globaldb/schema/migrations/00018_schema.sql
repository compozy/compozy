-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_token_stats" table
CREATE TABLE `new_token_stats` (`id` text NULL, `session_id` text NOT NULL, `agent_name` text NOT NULL, `input_tokens` integer NULL, `output_tokens` integer NULL, `total_tokens` integer NULL, `total_cost` real NULL, `cost_currency` text NULL, `cost_status` text NOT NULL DEFAULT 'unknown', `cost_source` text NOT NULL DEFAULT 'none', `turn_count` integer NOT NULL DEFAULT 0, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (cost_status IN ('actual', 'estimated', 'included', 'unknown')), CHECK (cost_source IN ('agent_reported', 'catalog_config', 'models_dev', 'builtin', 'none')));
-- copy rows from old table "token_stats" to new temporary table "new_token_stats"
INSERT INTO `new_token_stats` (`id`, `session_id`, `agent_name`, `input_tokens`, `output_tokens`, `total_tokens`, `total_cost`, `cost_currency`, `cost_status`, `cost_source`, `turn_count`, `updated_at`) SELECT `id`, `session_id`, `agent_name`, `input_tokens`, `output_tokens`, `total_tokens`, `total_cost`, `cost_currency`, CASE WHEN `total_cost` IS NULL THEN 'unknown' ELSE 'actual' END, CASE WHEN `total_cost` IS NULL THEN 'none' ELSE 'agent_reported' END, `turn_count`, `updated_at` FROM `token_stats`;
-- drop "token_stats" table after copying rows
DROP TABLE `token_stats`;
-- rename temporary table "new_token_stats" to "token_stats"
ALTER TABLE `new_token_stats` RENAME TO `token_stats`;
-- create index "idx_token_stats_session" to table: "token_stats"
CREATE INDEX `idx_token_stats_session` ON `token_stats` (`session_id`);
-- create index "idx_token_stats_session_agent" to table: "token_stats"
CREATE UNIQUE INDEX `idx_token_stats_session_agent` ON `token_stats` (`session_id`, `agent_name`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
