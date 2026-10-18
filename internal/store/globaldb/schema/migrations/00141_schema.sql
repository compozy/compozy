-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_gateway_device_sessions" table
CREATE TABLE `new_gateway_device_sessions` (`id` text NULL, `name` text NOT NULL, `token_hash` text NOT NULL, `actor_kind` text NOT NULL, `pairing_origin` text NOT NULL, `revoke_epoch` integer NOT NULL DEFAULT 0, `created_at` text NOT NULL, `last_seen_at` text NULL, `revoked_at` text NULL, PRIMARY KEY (`id`), CHECK (length(trim(name)) > 0), CHECK (length(token_hash) = 64), CHECK (actor_kind IN ('operator_device', 'cli_profile', 'app_token')), CHECK (length(trim(pairing_origin)) > 0), CHECK (revoke_epoch >= 0));
-- copy rows from old table "gateway_device_sessions" to new temporary table "new_gateway_device_sessions"
INSERT INTO `new_gateway_device_sessions` (`id`, `name`, `token_hash`, `actor_kind`, `pairing_origin`, `revoke_epoch`, `created_at`, `last_seen_at`, `revoked_at`) SELECT `id`, `name`, `token_hash`, `actor_kind`, `pairing_origin`, `revoke_epoch`, `created_at`, `last_seen_at`, `revoked_at` FROM `gateway_device_sessions`;
-- drop "gateway_device_sessions" table after copying rows
DROP TABLE `gateway_device_sessions`;
-- rename temporary table "new_gateway_device_sessions" to "gateway_device_sessions"
ALTER TABLE `new_gateway_device_sessions` RENAME TO `gateway_device_sessions`;
-- create index "gateway_device_sessions_token_hash" to table: "gateway_device_sessions"
CREATE UNIQUE INDEX `gateway_device_sessions_token_hash` ON `gateway_device_sessions` (`token_hash`);
-- create "gateway_device_origins" table
CREATE TABLE `gateway_device_origins` (`device_id` text NOT NULL, `origin` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`device_id`, `origin`), CONSTRAINT `0` FOREIGN KEY (`device_id`) REFERENCES `gateway_device_sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(origin) BETWEEN 8 AND 2048));
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
