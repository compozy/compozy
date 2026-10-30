-- +goose Up
-- add column "kernel_token" to table: "session_process_observations"
ALTER TABLE `session_process_observations` ADD COLUMN `kernel_token` text NOT NULL DEFAULT '';
