CREATE TABLE extensions (
		name          TEXT PRIMARY KEY,
		version       TEXT NOT NULL,
		source        TEXT NOT NULL,
		manifest_path TEXT NOT NULL,
		format        TEXT NOT NULL DEFAULT 'compozy',
		ingest_diagnostics_json TEXT NOT NULL DEFAULT '[]',
		installed_at  TEXT NOT NULL,
		provides_json TEXT NOT NULL DEFAULT '[]',
		permissions_json TEXT NOT NULL DEFAULT '[]',
		checksum      TEXT NOT NULL,
		registry_slug TEXT,
		registry_name TEXT,
		remote_version TEXT,
		provenance_json TEXT NOT NULL DEFAULT '{}',
		network_requirement_digest TEXT NOT NULL DEFAULT '',
		network_confirmed_by TEXT,
		network_confirmed_at TEXT
	);

CREATE TABLE extension_env_bindings (
		extension_name TEXT NOT NULL,
		profile_id TEXT NOT NULL DEFAULT '',
		workspace_id TEXT NOT NULL DEFAULT '',
		env_name TEXT NOT NULL,
		secret_ref TEXT NOT NULL,
		mcp_server TEXT NOT NULL DEFAULT '',
		header_name TEXT NOT NULL DEFAULT '',
		kind TEXT NOT NULL CHECK (kind = 'extension_env'),
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (extension_name, profile_id, workspace_id, env_name),
		CHECK ((mcp_server = '' AND header_name = '') OR (mcp_server <> '' AND header_name <> ''))
	);

CREATE INDEX idx_extension_env_bindings_secret_ref
	ON extension_env_bindings (secret_ref);

CREATE TRIGGER extension_env_bindings_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM extension_env_bindings WHERE workspace_id = OLD.id;
END;

CREATE TRIGGER extension_env_bindings_profile_delete
AFTER DELETE ON profiles
BEGIN
	DELETE FROM extension_env_bindings WHERE profile_id = OLD.id;
END;

CREATE TRIGGER extension_env_bindings_profile_insert
BEFORE INSERT ON extension_env_bindings
WHEN NEW.profile_id <> '' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id)
BEGIN
	SELECT RAISE(ABORT, 'profile_not_found');
END;
