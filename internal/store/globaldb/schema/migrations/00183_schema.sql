-- +goose Up
-- add column "unattended_counted" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `unattended_counted` integer NOT NULL DEFAULT 0;
