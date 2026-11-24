CREATE TABLE ingress_endpoint_state (
	id INTEGER NOT NULL PRIMARY KEY CHECK (id = 1),
	endpoint_url TEXT NOT NULL,
	endpoint_generation INTEGER NOT NULL CHECK (endpoint_generation > 0),
	reachable BOOLEAN NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL
);

CREATE TABLE ingress_bindings (
	subject_kind TEXT NOT NULL CHECK (subject_kind IN ('webhook_trigger')),
	subject_id TEXT NOT NULL CHECK (length(trim(subject_id)) > 0),
	scope_kind TEXT NOT NULL CHECK (scope_kind IN ('global', 'workspace')),
	workspace_id TEXT,
	endpoint_generation INTEGER NOT NULL CHECK (endpoint_generation > 0),
	confirmed_at TEXT NOT NULL,
	PRIMARY KEY (subject_kind, subject_id),
	CHECK (
		(scope_kind = 'global' AND workspace_id IS NULL) OR
		(scope_kind = 'workspace' AND workspace_id IS NOT NULL)
	)
);

CREATE INDEX ingress_bindings_workspace
	ON ingress_bindings(workspace_id, subject_kind, subject_id)
	WHERE workspace_id IS NOT NULL;

CREATE TRIGGER ingress_resource_delete
AFTER DELETE ON resource_records
WHEN OLD.kind = 'automation.trigger'
BEGIN
	DELETE FROM ingress_bindings
	WHERE subject_id = OLD.id
		AND subject_kind = 'webhook_trigger';
END;

CREATE TRIGGER ingress_trigger_resource_identity_update
AFTER UPDATE OF scope_kind, scope_id, spec_json ON resource_records
WHEN OLD.kind = 'automation.trigger' AND (
	OLD.scope_kind IS NOT NEW.scope_kind OR
	COALESCE(OLD.scope_id, '') <> COALESCE(NEW.scope_id, '') OR
	json_extract(OLD.spec_json, '$.event') IS NOT json_extract(NEW.spec_json, '$.event') OR
	json_extract(OLD.spec_json, '$.endpoint_slug') IS NOT json_extract(NEW.spec_json, '$.endpoint_slug') OR
	json_extract(OLD.spec_json, '$.webhook_id') IS NOT json_extract(NEW.spec_json, '$.webhook_id')
)
BEGIN
	DELETE FROM ingress_bindings
	WHERE subject_kind = 'webhook_trigger' AND subject_id = OLD.id;
END;
