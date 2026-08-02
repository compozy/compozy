-- +goose Up
-- Legacy cursor subjects fused scope and opaque event identifiers. They cannot
-- be decoded without ambiguity, so progress is deliberately restarted.
DROP TABLE notification_cursors;

CREATE TABLE notification_cursors (`scope_kind` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `consumer_id` text NOT NULL, `stream_name` text NOT NULL, `subject_id` text NOT NULL DEFAULT '', `last_sequence` integer NOT NULL DEFAULT 0, `last_delivery_id` text NOT NULL DEFAULT '', `last_delivered_at` text NULL, `last_error` text NOT NULL DEFAULT '', `updated_at` text NOT NULL, PRIMARY KEY (`scope_kind`, `workspace_id`, `consumer_id`, `stream_name`, `subject_id`), CHECK (scope_kind IN ('global', 'workspace')), CHECK (
				(scope_kind = 'global' AND workspace_id = '') OR
				(scope_kind = 'workspace' AND trim(workspace_id) <> '')
			), CHECK (trim(consumer_id) <> ''), CHECK (trim(stream_name) <> ''), CHECK (last_sequence >= 0));

CREATE INDEX `notification_cursors_stream_sequence_idx` ON `notification_cursors` (`scope_kind`, `workspace_id`, `stream_name`, `last_sequence` DESC)
	WHERE last_sequence > 0;
