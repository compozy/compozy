-- +goose Up
CREATE TABLE app_tokens (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL CHECK (length(trim(name)) > 0),
	token_hash TEXT NOT NULL CHECK (length(token_hash) = 64),
	revoke_epoch INTEGER NOT NULL DEFAULT 0 CHECK (revoke_epoch >= 0),
	created_at TEXT NOT NULL,
	last_seen_at TEXT,
	revoked_at TEXT
);

CREATE UNIQUE INDEX app_tokens_token_hash ON app_tokens(token_hash);

CREATE TABLE app_token_origins (
	app_token_id TEXT NOT NULL REFERENCES app_tokens(id) ON DELETE CASCADE,
	origin TEXT NOT NULL CHECK (length(origin) BETWEEN 8 AND 2048),
	created_at TEXT NOT NULL,
	PRIMARY KEY (app_token_id, origin)
);

INSERT INTO app_tokens (id, name, token_hash, revoke_epoch, created_at, last_seen_at, revoked_at)
SELECT id, name, token_hash, revoke_epoch, created_at, last_seen_at, revoked_at
FROM gateway_device_sessions WHERE actor_kind = 'app_token';

INSERT INTO app_token_origins (app_token_id, origin, created_at)
SELECT origins.device_id, origins.origin, origins.created_at
FROM gateway_device_origins AS origins
JOIN app_tokens ON app_tokens.id = origins.device_id;

DROP TRIGGER gateway_ingress_resource_delete;
DROP TRIGGER gateway_ingress_trigger_resource_identity_update;
ALTER TABLE gateway_ingress_bindings RENAME TO ingress_bindings;
DROP INDEX gateway_ingress_bindings_workspace;
CREATE INDEX ingress_bindings_workspace
	ON ingress_bindings(workspace_id, subject_kind, subject_id)
	WHERE workspace_id IS NOT NULL;

-- +goose StatementBegin
CREATE TRIGGER ingress_resource_delete
AFTER DELETE ON resource_records
WHEN OLD.kind = 'automation.trigger'
BEGIN
	DELETE FROM ingress_bindings
	WHERE subject_id = OLD.id
		AND subject_kind = 'webhook_trigger';
END;
-- +goose StatementEnd

-- +goose StatementBegin
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
-- +goose StatementEnd

INSERT OR IGNORE INTO ingress_endpoint_state
	(id, endpoint_url, endpoint_generation, reachable, source, updated_at)
SELECT 1, endpoint_url, endpoint_generation, reachable, 'gateway', updated_at
FROM gateway_endpoint_state WHERE tier = 'public';

CREATE TABLE migration_notices (
	key TEXT PRIMARY KEY,
	payload TEXT NOT NULL CHECK (json_valid(payload)),
	created_at TEXT NOT NULL,
	dismissed_at TEXT
);

WITH removed AS (
	SELECT
		(SELECT count(*) FROM gateway_device_sessions WHERE actor_kind <> 'app_token') AS devices,
		(SELECT count(*) FROM gateway_providers) AS providers,
		(SELECT count(*) FROM ingress_bindings
		 WHERE endpoint_generation < (SELECT endpoint_generation FROM ingress_endpoint_state WHERE id = 1)
		    OR NOT EXISTS (SELECT 1 FROM ingress_endpoint_state WHERE id = 1)) AS bindings
)
INSERT INTO migration_notices (key, payload, created_at)
SELECT 'gateway_removed',
	json_object('devices', devices, 'providers', providers, 'connection_profiles', 0,
		'bindings_needing_confirmation', bindings),
	strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM removed
WHERE devices > 0 OR providers > 0 OR bindings > 0
   OR (EXISTS (SELECT 1 FROM ingress_endpoint_state WHERE source = 'gateway')
       AND EXISTS (SELECT 1 FROM ingress_bindings));

-- Retire the shipped provider's enrollment so it cannot grant bundled trust.
DELETE FROM extensions
WHERE name = 'tailscale' AND source = 'bundled'
  AND provides_json = '["connectivity.provider"]';

ALTER TABLE ingress_endpoint_state DROP COLUMN source;
ALTER TABLE extensions DROP COLUMN gateway_requirement_digest;
ALTER TABLE extensions DROP COLUMN gateway_confirmed_by;
ALTER TABLE extensions DROP COLUMN gateway_confirmed_at;
ALTER TABLE extension_dev_links DROP COLUMN gateway_requirement_digest;
ALTER TABLE extension_dev_links DROP COLUMN gateway_confirmed_by;
ALTER TABLE extension_dev_links DROP COLUMN gateway_confirmed_at;

DROP TABLE gateway_device_origins;
DROP TABLE gateway_device_sessions;
DROP TABLE gateway_provider_activations;
DROP TABLE gateway_providers;
DROP TABLE gateway_surface_exposure;
DROP TABLE gateway_endpoint_state;
