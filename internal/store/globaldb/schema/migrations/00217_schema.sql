-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_agent_instance_daily_budget_recovery" table
CREATE TABLE `new_agent_instance_daily_budget_recovery` (`instance_id` text NOT NULL, `generation` integer NOT NULL, `pending` integer NOT NULL, `latched` integer NOT NULL DEFAULT 0, PRIMARY KEY (`instance_id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation > 0), CHECK (pending IN (0, 1)), CHECK (latched IN (0, 1)));
-- copy rows from old table "agent_instance_daily_budget_recovery" to new temporary table "new_agent_instance_daily_budget_recovery"
INSERT INTO `new_agent_instance_daily_budget_recovery` (`instance_id`, `generation`, `pending`, `latched`) SELECT `instance_id`, `generation`, `pending`, `latched` FROM `agent_instance_daily_budget_recovery`;
-- drop "agent_instance_daily_budget_recovery" table after copying rows
DROP TABLE `agent_instance_daily_budget_recovery`;
-- rename temporary table "new_agent_instance_daily_budget_recovery" to "agent_instance_daily_budget_recovery"
ALTER TABLE `new_agent_instance_daily_budget_recovery` RENAME TO `agent_instance_daily_budget_recovery`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
