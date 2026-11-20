-- +goose Up
-- add column "app_ingress" to table: "session_prompt_admissions"
ALTER TABLE `session_prompt_admissions` ADD COLUMN `app_ingress` integer NOT NULL DEFAULT 1;
-- add column "app_ingress" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `app_ingress` integer NOT NULL DEFAULT 1;
-- Preserve the recorded human ingress classification of existing work.
UPDATE session_prompt_admissions SET app_ingress = 0 WHERE human_ingress = 1;
UPDATE session_input_queue SET app_ingress = 0 WHERE human_ingress = 1;
-- add column "ownership_unresolved" to table: "token_usage_receipts"
ALTER TABLE `token_usage_receipts` ADD COLUMN `ownership_unresolved` integer NOT NULL DEFAULT 0;
-- create "agent_instance_daily_budget_uncertain_days" table
CREATE TABLE `agent_instance_daily_budget_uncertain_days` (`instance_id` text NOT NULL, `day` text NOT NULL, `source_session_id` text NOT NULL, `recorded_at` text NOT NULL, PRIMARY KEY (`instance_id`, `day`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(day) = 10));
-- create "agent_instance_daily_budget_recovery" table
CREATE TABLE `agent_instance_daily_budget_recovery` (`instance_id` text NOT NULL, `generation` integer NOT NULL, `pending` integer NOT NULL, `latched` integer NOT NULL DEFAULT 0, PRIMARY KEY (`instance_id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation > 0), CHECK (pending IN (0, 1)), CHECK (latched IN (0, 1)));
-- create "daily_budget_recovery_cursors" table
CREATE TABLE `daily_budget_recovery_cursors` (`session_id` text NOT NULL, `after_sequence` integer NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`session_id`), CHECK (after_sequence >= 0));
