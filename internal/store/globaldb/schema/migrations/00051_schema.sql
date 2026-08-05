-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_notification_cursors" table
CREATE TABLE `new_notification_cursors` (`scope_kind` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `consumer_id` text NOT NULL, `stream_name` text NOT NULL, `subject_id` text NOT NULL DEFAULT '', `last_sequence` integer NOT NULL DEFAULT 0, `last_delivery_id` text NOT NULL DEFAULT '', `last_delivered_at` text NULL, `last_error` text NOT NULL DEFAULT '', `updated_at` text NOT NULL, PRIMARY KEY (`scope_kind`, `workspace_id`, `consumer_id`, `stream_name`, `subject_id`), CHECK (scope_kind IN ('global', 'workspace')), CHECK (
				(scope_kind = 'global' AND workspace_id = '') OR
(scope_kind = 'workspace' AND workspace_id <> '')
			), CHECK (consumer_id <> ''), CHECK (stream_name <> ''), CHECK (last_sequence >= 0));
-- copy rows from old table "notification_cursors" to new temporary table "new_notification_cursors"
INSERT INTO `new_notification_cursors` (`scope_kind`, `workspace_id`, `consumer_id`, `stream_name`, `subject_id`, `last_sequence`, `last_delivery_id`, `last_delivered_at`, `last_error`, `updated_at`) SELECT `scope_kind`, `workspace_id`, `consumer_id`, `stream_name`, `subject_id`, `last_sequence`, `last_delivery_id`, `last_delivered_at`, `last_error`, `updated_at` FROM `notification_cursors`;
-- drop "notification_cursors" table after copying rows
DROP TABLE `notification_cursors`;
-- rename temporary table "new_notification_cursors" to "notification_cursors"
ALTER TABLE `new_notification_cursors` RENAME TO `notification_cursors`;
-- create index "notification_cursors_stream_sequence_idx" to table: "notification_cursors"
CREATE INDEX `notification_cursors_stream_sequence_idx` ON `notification_cursors` (`scope_kind`, `workspace_id`, `stream_name`, `last_sequence` DESC) WHERE last_sequence > 0;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
