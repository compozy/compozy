-- +goose Up
-- create "bridge_deliveries" table
CREATE TABLE `bridge_deliveries` (`delivery_id` text NULL, `session_id` text NOT NULL, `turn_id` text NOT NULL, `routing_key` text NOT NULL, `bridge_instance_id` text NOT NULL, `scope` text NOT NULL, `workspace_id` text NULL, `state` text NOT NULL, `last_sent_seq` integer NOT NULL DEFAULT 0, `last_acked_seq` integer NOT NULL DEFAULT 0, `remote_message_id` text NULL, `terminal_error` text NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`delivery_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`bridge_instance_id`) REFERENCES `bridge_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(delivery_id)) > 0), CHECK (length(trim(session_id)) > 0), CHECK (length(trim(turn_id)) > 0), CHECK (
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
-- create index "idx_bridge_deliveries_scope" to table: "bridge_deliveries"
CREATE INDEX `idx_bridge_deliveries_scope` ON `bridge_deliveries` (`scope`, `workspace_id`, `state`, `updated_at`, `delivery_id`);
-- create index "idx_bridge_deliveries_instance" to table: "bridge_deliveries"
CREATE INDEX `idx_bridge_deliveries_instance` ON `bridge_deliveries` (`bridge_instance_id`, `state`, `updated_at`, `delivery_id`);
-- create "bridge_delivery_metrics" table
CREATE TABLE `bridge_delivery_metrics` (`bridge_instance_id` text NULL, `scope` text NOT NULL, `workspace_id` text NULL, `delivery_dropped_total` integer NOT NULL DEFAULT 0, `delivery_dropped_by_reason_json` text NOT NULL DEFAULT '{}', `delivery_failures_total` integer NOT NULL DEFAULT 0, `last_error` text NULL, `last_error_at` text NULL, `last_success_at` text NULL, `updated_at` text NOT NULL, PRIMARY KEY (`bridge_instance_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`bridge_instance_id`) REFERENCES `bridge_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (scope IN ('global', 'workspace')), CHECK (delivery_dropped_total >= 0), CHECK (
				json_valid(delivery_dropped_by_reason_json) AND
				json_type(delivery_dropped_by_reason_json) = 'object'
			), CHECK (delivery_failures_total >= 0), CHECK (length(trim(updated_at)) > 0), CHECK (
				(scope = 'global' AND workspace_id IS NULL) OR
				(scope = 'workspace' AND workspace_id IS NOT NULL)
			), CHECK (
				(last_error IS NULL AND last_error_at IS NULL) OR
				(length(trim(last_error)) > 0 AND last_error_at IS NOT NULL)
			));
-- create index "idx_bridge_delivery_metrics_scope" to table: "bridge_delivery_metrics"
CREATE INDEX `idx_bridge_delivery_metrics_scope` ON `bridge_delivery_metrics` (`scope`, `workspace_id`, `updated_at`, `bridge_instance_id`);

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

-- +goose Down
-- reverse: create trigger "trg_bridge_instance_active_delivery_identity"
DROP TRIGGER `trg_bridge_instance_active_delivery_identity`;
-- reverse: create trigger "trg_bridge_instance_active_delivery_delete"
DROP TRIGGER `trg_bridge_instance_active_delivery_delete`;
-- reverse: create index "idx_bridge_delivery_metrics_scope" to table: "bridge_delivery_metrics"
DROP INDEX `idx_bridge_delivery_metrics_scope`;
-- reverse: create "bridge_delivery_metrics" table
DROP TABLE `bridge_delivery_metrics`;
-- reverse: create index "idx_bridge_deliveries_instance" to table: "bridge_deliveries"
DROP INDEX `idx_bridge_deliveries_instance`;
-- reverse: create index "idx_bridge_deliveries_scope" to table: "bridge_deliveries"
DROP INDEX `idx_bridge_deliveries_scope`;
-- reverse: create "bridge_deliveries" table
DROP TABLE `bridge_deliveries`;
