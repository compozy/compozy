-- +goose Up
-- add column "pinned_at" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `pinned_at` text NULL;
-- create index "idx_sessions_pinned" to table: "sessions"
CREATE INDEX `idx_sessions_pinned` ON `sessions` (`pinned_at`, `id`) WHERE pinned_at IS NOT NULL;
