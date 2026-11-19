-- +goose Up
-- create "agent_instance_daily_budget_limits" table
CREATE TABLE `agent_instance_daily_budget_limits` (`instance_id` text NOT NULL, `token_cap` integer NULL, `usd_cap` text NULL, `updated_at` text NOT NULL, PRIMARY KEY (`instance_id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (token_cap IS NULL OR token_cap >= 0), CHECK (usd_cap IS NULL OR length(trim(usd_cap)) > 0));
-- create "agent_instance_daily_budget_notices" table
CREATE TABLE `agent_instance_daily_budget_notices` (`instance_id` text NOT NULL, `day` text NOT NULL, `issued_at` text NOT NULL, PRIMARY KEY (`instance_id`, `day`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(day) = 10));
