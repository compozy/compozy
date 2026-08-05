-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_bridge_deliveries" table
CREATE TABLE `new_bridge_deliveries` (`delivery_id` text NULL, `session_id` text NOT NULL, `turn_id` text NOT NULL, `routing_key` text NOT NULL, `bridge_instance_id` text NOT NULL, `scope` text NOT NULL, `workspace_id` text NULL, `state` text NOT NULL, `last_sent_seq` integer NOT NULL DEFAULT 0, `last_acked_seq` integer NOT NULL DEFAULT 0, `remote_message_id` text NULL, `terminal_error` text NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`delivery_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`bridge_instance_id`) REFERENCES `bridge_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(delivery_id) > 0), CHECK (length(trim(session_id)) > 0), CHECK (length(trim(turn_id)) > 0), CHECK (
				json_valid(routing_key) AND json_type(routing_key) = 'object'
			), CHECK (scope IN ('global', 'workspace')), CHECK (state IN ('active', 'terminal_ok', 'terminal_error')), CHECK (last_sent_seq >= 0), CHECK (
				last_acked_seq >= 0 AND last_acked_seq <= last_sent_seq
			), CHECK (length(trim(created_at)) > 0), CHECK (length(trim(updated_at)) > 0), CHECK (
				(scope = 'global' AND workspace_id IS NULL) OR
				(scope = 'workspace' AND workspace_id IS NOT NULL)
			), CHECK (
				(state = 'terminal_error' AND terminal_error IS NOT NULL AND length(trim(terminal_error)) > 0) OR
				(state != 'terminal_error' AND terminal_error IS NULL)
			));
-- copy rows from old table "bridge_deliveries" to new temporary table "new_bridge_deliveries"
INSERT INTO `new_bridge_deliveries` (`delivery_id`, `session_id`, `turn_id`, `routing_key`, `bridge_instance_id`, `scope`, `workspace_id`, `state`, `last_sent_seq`, `last_acked_seq`, `remote_message_id`, `terminal_error`, `created_at`, `updated_at`) SELECT `delivery_id`, `session_id`, `turn_id`, `routing_key`, `bridge_instance_id`, `scope`, `workspace_id`, `state`, `last_sent_seq`, `last_acked_seq`, `remote_message_id`, `terminal_error`, `created_at`, `updated_at` FROM `bridge_deliveries`;
-- SQLite drops triggers that refer to a rebuilt table before the old table is removed.
DROP TRIGGER `trg_bridge_instance_active_delivery_delete`;
DROP TRIGGER `trg_bridge_instance_active_delivery_identity`;
-- drop "bridge_deliveries" table after copying rows
DROP TABLE `bridge_deliveries`;
-- rename temporary table "new_bridge_deliveries" to "bridge_deliveries"
ALTER TABLE `new_bridge_deliveries` RENAME TO `bridge_deliveries`;
-- create index "idx_bridge_deliveries_scope" to table: "bridge_deliveries"
CREATE INDEX `idx_bridge_deliveries_scope` ON `bridge_deliveries` (`scope`, `workspace_id`, `state`, `updated_at`, `delivery_id`);
-- create index "idx_bridge_deliveries_instance" to table: "bridge_deliveries"
CREATE INDEX `idx_bridge_deliveries_instance` ON `bridge_deliveries` (`bridge_instance_id`, `state`, `updated_at`, `delivery_id`);
-- +goose StatementBegin
CREATE TRIGGER trg_bridge_instance_active_delivery_delete
		BEFORE DELETE ON bridge_instances
		WHEN EXISTS (
			SELECT 1 FROM bridge_deliveries
			WHERE bridge_instance_id = OLD.id AND state = 'active'
		)
		BEGIN
			SELECT RAISE(ABORT, 'bridge instance has active deliveries');
		END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_bridge_instance_active_delivery_identity
		BEFORE UPDATE OF scope, workspace_id, platform, extension_name ON bridge_instances
		WHEN EXISTS (
			SELECT 1 FROM bridge_deliveries
			WHERE bridge_instance_id = OLD.id AND state = 'active'
		) AND (
			NEW.scope IS NOT OLD.scope OR
			NEW.workspace_id IS NOT OLD.workspace_id OR
			NEW.platform IS NOT OLD.platform OR
			NEW.extension_name IS NOT OLD.extension_name
		)
		BEGIN
			SELECT RAISE(ABORT, 'active delivery locks bridge instance identity');
		END;
-- +goose StatementEnd
-- create "new_network_wake_events" table
CREATE TABLE `new_network_wake_events` (`sequence` integer NULL PRIMARY KEY AUTOINCREMENT, `workspace_id` text NOT NULL, `wake_id` text NOT NULL, `task_run_id` text NOT NULL, `owner_key` text NOT NULL, `target_session_id` text NOT NULL, `event_type` text NOT NULL, `state` text NOT NULL, `claim_token_hash` text NOT NULL DEFAULT '', `usage_state` text NOT NULL DEFAULT '', `actual_wall_ms` integer NULL, `input_tokens` integer NULL, `output_tokens` integer NULL, `reason` text NOT NULL DEFAULT '', `actor_kind` text NOT NULL, `actor_ref` text NOT NULL, `timestamp` text NOT NULL, CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `target_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `task_run_id`) REFERENCES `task_runs` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`workspace_id`, `wake_id`, `owner_key`) REFERENCES `network_live_wakes` (`workspace_id`, `wake_id`, `owner_key`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `3` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(owner_key)) > 0), CHECK (
		event_type IN ('admitted', 'claimed', 'heartbeat', 'released', 'recovered', 'settled')
	), CHECK (usage_state IN ('', 'actual', 'usage_unavailable')), CHECK (actual_wall_ms IS NULL OR actual_wall_ms >= 0), CHECK (input_tokens IS NULL OR input_tokens >= 0), CHECK (output_tokens IS NULL OR output_tokens >= 0));
-- copy rows from old table "network_wake_events" to new temporary table "new_network_wake_events"
INSERT INTO `new_network_wake_events` (`sequence`, `workspace_id`, `wake_id`, `task_run_id`, `owner_key`, `target_session_id`, `event_type`, `state`, `claim_token_hash`, `usage_state`, `actual_wall_ms`, `input_tokens`, `output_tokens`, `reason`, `actor_kind`, `actor_ref`, `timestamp`) SELECT `sequence`, `workspace_id`, `wake_id`, `task_run_id`, `owner_key`, `target_session_id`, `event_type`, `state`, `claim_token_hash`, `usage_state`, `actual_wall_ms`, `input_tokens`, `output_tokens`, `reason`, `actor_kind`, `actor_ref`, `timestamp` FROM `network_wake_events`;
-- drop "network_wake_events" table after copying rows
DROP TABLE `network_wake_events`;
-- rename temporary table "new_network_wake_events" to "network_wake_events"
ALTER TABLE `new_network_wake_events` RENAME TO `network_wake_events`;
-- create index "idx_network_wake_events_wake_sequence" to table: "network_wake_events"
CREATE INDEX `idx_network_wake_events_wake_sequence` ON `network_wake_events` (`workspace_id`, `wake_id`, `sequence`);
-- create "new_notification_cursors" table
CREATE TABLE `new_notification_cursors` (`scope_kind` text NOT NULL, `workspace_id` text NOT NULL DEFAULT '', `consumer_id` text NOT NULL, `stream_name` text NOT NULL, `subject_id` text NOT NULL DEFAULT '', `last_sequence` integer NOT NULL DEFAULT 0, `last_delivery_id` text NOT NULL DEFAULT '', `last_delivered_at` text NULL, `last_error` text NOT NULL DEFAULT '', `updated_at` text NOT NULL, PRIMARY KEY (`scope_kind`, `workspace_id`, `consumer_id`, `stream_name`, `subject_id`), CHECK (scope_kind IN ('global', 'workspace')), CHECK (
				(scope_kind = 'global' AND workspace_id = '') OR
(scope_kind = 'workspace' AND workspace_id <> '')
			), CHECK (trim(consumer_id) <> ''), CHECK (trim(stream_name) <> ''), CHECK (last_sequence >= 0));
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
