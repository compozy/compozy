-- +goose Up
-- add column "follow_up_after" to table: "session_prompt_reply_watches"
ALTER TABLE `session_prompt_reply_watches` ADD COLUMN `follow_up_after` integer NOT NULL DEFAULT 0;
-- add column "interim_wait" to table: "session_prompt_reply_watches"
ALTER TABLE `session_prompt_reply_watches` ADD COLUMN `interim_wait` text NULL;
