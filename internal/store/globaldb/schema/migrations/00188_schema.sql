-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_heartbeat_source_script_approvals" table
CREATE TABLE `new_heartbeat_source_script_approvals` (`profile_id` text NOT NULL, `definition_dir` text NOT NULL, `script_name` text NOT NULL, `sha256` text NOT NULL, `approved_at` text NOT NULL, `approved_via` text NOT NULL, PRIMARY KEY (`profile_id`, `definition_dir`, `script_name`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (approved_via IN ('cli', 'web', 'http', 'uds')));
-- copy rows from old table "heartbeat_source_script_approvals" to new temporary table "new_heartbeat_source_script_approvals"
INSERT INTO `new_heartbeat_source_script_approvals` (`profile_id`, `definition_dir`, `script_name`, `sha256`, `approved_at`, `approved_via`) SELECT `profile_id`, `definition_dir`, `script_name`, `sha256`, `approved_at`, `approved_via` FROM `heartbeat_source_script_approvals`;
-- drop "heartbeat_source_script_approvals" table after copying rows
DROP TABLE `heartbeat_source_script_approvals`;
-- rename temporary table "new_heartbeat_source_script_approvals" to "heartbeat_source_script_approvals"
ALTER TABLE `new_heartbeat_source_script_approvals` RENAME TO `heartbeat_source_script_approvals`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
