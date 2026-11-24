-- +goose Up
-- add column "signing_key_id" to table: "remote_sessions"
ALTER TABLE `remote_sessions` ADD COLUMN `signing_key_id` text NOT NULL DEFAULT '';
