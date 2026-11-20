-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_session_prompt_reply_watches" table
CREATE TABLE `new_session_prompt_reply_watches` (`id` text NOT NULL, `workspace_id` text NOT NULL, `sender_session_id` text NOT NULL, `target_workspace_id` text NOT NULL, `target_session_id` text NOT NULL, `message_id` text NOT NULL, `admission_id` text NOT NULL, `turn_id` text NULL, `queue_entry_id` text NULL, `delivered_input_id` text NULL, `abandon_reason` text NULL, `hop` integer NOT NULL, `state` text NOT NULL, `outcome` text NULL, `reply_text` text NULL, `reply_truncated` integer NOT NULL DEFAULT 0, `created_at` text NOT NULL, `fired_at` text NULL, `delivered_at` text NULL, `follow_up_after` integer NOT NULL DEFAULT 0, `interim_wait` text NULL, PRIMARY KEY (`id`), CHECK (abandon_reason IN ('sender_gone', 'send_failed', 'superseded')), CHECK (state IN ('armed', 'fired', 'delivered', 'abandoned')), CHECK (outcome IN ('completed', 'failed', 'canceled', 'dropped', 'unknown', 'expired')));
-- copy rows from old table "session_prompt_reply_watches" to new temporary table "new_session_prompt_reply_watches"
INSERT INTO `new_session_prompt_reply_watches` (`rowid`, `id`, `workspace_id`, `sender_session_id`, `target_workspace_id`, `target_session_id`, `message_id`, `admission_id`, `turn_id`, `queue_entry_id`, `delivered_input_id`, `abandon_reason`, `hop`, `state`, `outcome`, `reply_text`, `reply_truncated`, `created_at`, `fired_at`, `delivered_at`, `follow_up_after`, `interim_wait`) SELECT `rowid`, `id`, `workspace_id`, `sender_session_id`, `target_workspace_id`, `target_session_id`, `message_id`, `admission_id`, `turn_id`, `queue_entry_id`, `delivered_input_id`, `abandon_reason`, `hop`, `state`, `outcome`, `reply_text`, `reply_truncated`, `created_at`, `fired_at`, `delivered_at`, `follow_up_after`, `interim_wait` FROM `session_prompt_reply_watches`;
-- drop "session_prompt_reply_watches" table after copying rows
DROP TABLE `session_prompt_reply_watches`;
-- rename temporary table "new_session_prompt_reply_watches" to "session_prompt_reply_watches"
ALTER TABLE `new_session_prompt_reply_watches` RENAME TO `session_prompt_reply_watches`;
-- create index "session_prompt_reply_watches_target_session_id_message_id" to table: "session_prompt_reply_watches"
CREATE UNIQUE INDEX `session_prompt_reply_watches_target_session_id_message_id` ON `session_prompt_reply_watches` (`target_session_id`, `message_id`);
-- create index "idx_reply_watches_sender_state" to table: "session_prompt_reply_watches"
CREATE INDEX `idx_reply_watches_sender_state` ON `session_prompt_reply_watches` (`sender_session_id`, `state`);
-- create index "idx_reply_watches_target_turn" to table: "session_prompt_reply_watches"
CREATE INDEX `idx_reply_watches_target_turn` ON `session_prompt_reply_watches` (`target_session_id`, `turn_id`);
-- create index "idx_reply_watches_target_queue" to table: "session_prompt_reply_watches"
CREATE INDEX `idx_reply_watches_target_queue` ON `session_prompt_reply_watches` (`target_session_id`, `queue_entry_id`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
