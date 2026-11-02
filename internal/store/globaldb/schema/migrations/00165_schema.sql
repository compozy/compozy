-- +goose Up
-- add column "expires_at" to table: "session_pending_interactions"
ALTER TABLE `session_pending_interactions` ADD COLUMN `expires_at` text NULL;
