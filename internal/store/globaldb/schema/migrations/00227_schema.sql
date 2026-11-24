-- +goose Up
-- add column "gate_policy_jws" to table: "installation_account"
ALTER TABLE `installation_account` ADD COLUMN `gate_policy_jws` text NULL;
-- add column "gate_policy_keys" to table: "installation_account"
ALTER TABLE `installation_account` ADD COLUMN `gate_policy_keys` text NULL;
-- add column "gate_policy_fetched_at" to table: "installation_account"
ALTER TABLE `installation_account` ADD COLUMN `gate_policy_fetched_at` text NULL;
-- add column "offline_grace_started_at" to table: "installation_account"
ALTER TABLE `installation_account` ADD COLUMN `offline_grace_started_at` text NULL;
-- add column "token_name" to table: "installation_account"
ALTER TABLE `installation_account` ADD COLUMN `token_name` text NULL;
