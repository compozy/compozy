-- +goose Up
-- create "gateway_endpoint_state" table
CREATE TABLE `gateway_endpoint_state` (`tier` text NOT NULL, `endpoint_url` text NOT NULL, `endpoint_generation` integer NOT NULL, `reachable` boolean NOT NULL DEFAULT 0, `updated_at` text NOT NULL, PRIMARY KEY (`tier`), CHECK (tier IN ('private', 'public')), CHECK (length(trim(endpoint_url)) > 0), CHECK (endpoint_generation > 0));
-- create "gateway_ingress_bindings" table
CREATE TABLE `gateway_ingress_bindings` (`subject_kind` text NOT NULL, `subject_id` text NOT NULL, `scope_kind` text NOT NULL, `workspace_id` text NULL, `endpoint_generation` integer NOT NULL, `confirmed_at` text NOT NULL, PRIMARY KEY (`subject_kind`, `subject_id`), CHECK (subject_kind IN ('webhook_trigger', 'bridge_instance')), CHECK (length(trim(subject_id)) > 0), CHECK (scope_kind IN ('global', 'workspace')), CHECK (endpoint_generation > 0), CHECK (
		(scope_kind = 'global' AND workspace_id IS NULL) OR
		(scope_kind = 'workspace' AND workspace_id IS NOT NULL)
	));
-- create index "gateway_ingress_bindings_workspace" to table: "gateway_ingress_bindings"
CREATE INDEX `gateway_ingress_bindings_workspace` ON `gateway_ingress_bindings` (`workspace_id`, `subject_kind`, `subject_id`) WHERE workspace_id IS NOT NULL;
-- create trigger "gateway_ingress_resource_delete"
-- +goose StatementBegin
CREATE TRIGGER gateway_ingress_resource_delete
AFTER DELETE ON resource_records
WHEN OLD.kind IN ('automation.trigger', 'bridge.instance')
BEGIN
	DELETE FROM gateway_ingress_bindings
	WHERE subject_id = OLD.id
		AND subject_kind = CASE OLD.kind
			WHEN 'automation.trigger' THEN 'webhook_trigger'
			WHEN 'bridge.instance' THEN 'bridge_instance'
		END;
END;
-- +goose StatementEnd
-- create trigger "gateway_ingress_trigger_resource_identity_update"
-- +goose StatementBegin
CREATE TRIGGER gateway_ingress_trigger_resource_identity_update
AFTER UPDATE OF scope_kind, scope_id, spec_json ON resource_records
WHEN OLD.kind = 'automation.trigger' AND (
	OLD.scope_kind IS NOT NEW.scope_kind OR
	COALESCE(OLD.scope_id, '') <> COALESCE(NEW.scope_id, '') OR
	json_extract(OLD.spec_json, '$.event') IS NOT json_extract(NEW.spec_json, '$.event') OR
	json_extract(OLD.spec_json, '$.endpoint_slug') IS NOT json_extract(NEW.spec_json, '$.endpoint_slug') OR
	json_extract(OLD.spec_json, '$.webhook_id') IS NOT json_extract(NEW.spec_json, '$.webhook_id')
)
BEGIN
	DELETE FROM gateway_ingress_bindings
	WHERE subject_kind = 'webhook_trigger' AND subject_id = OLD.id;
END;
-- +goose StatementEnd
-- create trigger "gateway_ingress_bridge_resource_identity_update"
-- +goose StatementBegin
CREATE TRIGGER gateway_ingress_bridge_resource_identity_update
AFTER UPDATE OF scope_kind, scope_id, spec_json ON resource_records
WHEN OLD.kind = 'bridge.instance' AND (
	OLD.scope_kind IS NOT NEW.scope_kind OR
	COALESCE(OLD.scope_id, '') <> COALESCE(NEW.scope_id, '') OR
	json_extract(OLD.spec_json, '$.provider_config') IS NOT
		json_extract(NEW.spec_json, '$.provider_config')
)
BEGIN
	DELETE FROM gateway_ingress_bindings
	WHERE subject_kind = 'bridge_instance' AND subject_id = OLD.id;
END;
-- +goose StatementEnd
