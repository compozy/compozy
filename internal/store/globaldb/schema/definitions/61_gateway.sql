CREATE TABLE gateway_device_sessions (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL CHECK (length(trim(name)) > 0),
	token_hash TEXT NOT NULL CHECK (length(token_hash) = 64),
	actor_kind TEXT NOT NULL CHECK (actor_kind IN ('operator_device', 'cli_profile')),
	pairing_origin TEXT NOT NULL CHECK (length(trim(pairing_origin)) > 0),
	revoke_epoch INTEGER NOT NULL DEFAULT 0 CHECK (revoke_epoch >= 0),
	created_at TEXT NOT NULL,
	last_seen_at TEXT,
	revoked_at TEXT
);

CREATE UNIQUE INDEX gateway_device_sessions_token_hash
	ON gateway_device_sessions(token_hash);

CREATE TABLE gateway_providers (
	name TEXT PRIMARY KEY CHECK (length(trim(name)) > 0),
	install_source TEXT NOT NULL CHECK (length(trim(install_source)) > 0),
	digest_confirmed TEXT,
	confirmed_at TEXT
);

CREATE TABLE gateway_provider_activations (
	provider_name TEXT NOT NULL REFERENCES gateway_providers(name) ON DELETE CASCADE,
	tier TEXT NOT NULL CHECK (tier IN ('private', 'public')),
	desired_state TEXT NOT NULL CHECK (desired_state IN ('enabled', 'disabled')),
	observed_state TEXT NOT NULL CHECK (observed_state IN ('down', 'establishing', 'up', 'degraded')),
	generation INTEGER NOT NULL DEFAULT 0 CHECK (generation >= 0),
	last_health_at TEXT,
	last_error TEXT,
	PRIMARY KEY (provider_name, tier)
);

CREATE UNIQUE INDEX gateway_active_provider_per_tier
	ON gateway_provider_activations(tier)
	WHERE desired_state = 'enabled';

CREATE TABLE gateway_surface_exposure (
	surface TEXT NOT NULL CHECK (surface IN ('operator_ui', 'webhook_ingress')),
	tier TEXT NOT NULL CHECK (tier IN ('private', 'public')),
	desired_state TEXT NOT NULL CHECK (desired_state IN ('enabled', 'disabled')),
	observed_state TEXT NOT NULL CHECK (observed_state IN ('off', 'on')),
	generation INTEGER NOT NULL DEFAULT 0 CHECK (generation >= 0),
	consented_at TEXT,
	PRIMARY KEY (surface, tier),
	CHECK (surface <> 'operator_ui' OR tier <> 'public' OR desired_state = 'disabled' OR consented_at IS NOT NULL)
);

CREATE TABLE gateway_endpoint_state (
	tier TEXT NOT NULL PRIMARY KEY CHECK (tier IN ('private', 'public')),
	endpoint_url TEXT NOT NULL CHECK (length(trim(endpoint_url)) > 0),
	endpoint_generation INTEGER NOT NULL CHECK (endpoint_generation > 0),
	reachable BOOLEAN NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL
);

CREATE TABLE gateway_ingress_bindings (
	subject_kind TEXT NOT NULL CHECK (subject_kind IN ('webhook_trigger', 'bridge_instance')),
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

CREATE INDEX gateway_ingress_bindings_workspace
	ON gateway_ingress_bindings(workspace_id, subject_kind, subject_id)
	WHERE workspace_id IS NOT NULL;

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
