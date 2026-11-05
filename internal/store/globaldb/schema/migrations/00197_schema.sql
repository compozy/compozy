-- +goose Up
-- add column "superseded_by" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `superseded_by` text NULL;
