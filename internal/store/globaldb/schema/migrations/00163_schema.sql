-- +goose Up
-- add column "owner_check_failed_at" to table: "session_monitors"
ALTER TABLE `session_monitors` ADD COLUMN `owner_check_failed_at` text NOT NULL DEFAULT '';
