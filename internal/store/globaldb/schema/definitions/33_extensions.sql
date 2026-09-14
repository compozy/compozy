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
		lifecycle_token TEXT NOT NULL DEFAULT '',
		registry_slug TEXT,
		registry_name TEXT,
		remote_version TEXT,
		provenance_json TEXT NOT NULL DEFAULT '{}',
		network_requirement_digest TEXT NOT NULL DEFAULT '',
		network_confirmed_by TEXT,
		network_confirmed_at TEXT
	);

CREATE TRIGGER extensions_profile_enablement_delete
AFTER DELETE ON extensions
WHEN NOT EXISTS (
	SELECT 1 FROM extension_dev_links WHERE extension_name = OLD.name
)
BEGIN
	DELETE FROM extension_profile_enablement WHERE extension_name = OLD.name;
END;

CREATE TABLE extension_env_bindings (
		extension_name TEXT NOT NULL,
		profile_id TEXT NOT NULL DEFAULT '',
		workspace_id TEXT NOT NULL DEFAULT '',
		env_name TEXT NOT NULL,
		secret_ref TEXT NOT NULL,
		input_id TEXT NOT NULL DEFAULT '',
		active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
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

CREATE TABLE extension_inputs (
 extension TEXT NOT NULL CHECK (trim(extension) <> ''),
 profile TEXT NOT NULL DEFAULT '',
 workspace_id TEXT NOT NULL DEFAULT '',
 input_id TEXT NOT NULL CHECK (trim(input_id) <> ''),
 type TEXT NOT NULL CHECK (type IN ('string', 'identifier', 'boolean')),
 value_json TEXT NOT NULL CHECK (json_valid(value_json)),
 active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
 updated_at TEXT NOT NULL,
 PRIMARY KEY (extension, profile, workspace_id, input_id)
);

CREATE TABLE extension_mcp_overrides (
 extension TEXT NOT NULL CHECK (trim(extension) <> ''),
 profile TEXT NOT NULL DEFAULT '',
 workspace_id TEXT NOT NULL DEFAULT '',
 server TEXT NOT NULL CHECK (trim(server) <> ''),
 runtime_name TEXT NOT NULL CHECK (trim(runtime_name) <> ''),
 env_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(env_json)),
 headers_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(headers_json)),
 url TEXT NOT NULL DEFAULT '',
 updated_at TEXT NOT NULL,
 PRIMARY KEY (extension, profile, workspace_id, server),
 UNIQUE (profile, workspace_id, runtime_name)
);

-- A package is stored once; installations grant its use to an exact profile/workspace
-- or, with empty selectors, to the existing global/all-profiles scope.
CREATE TABLE extension_installations (
 extension_name TEXT NOT NULL REFERENCES extensions(name) ON DELETE CASCADE,
 profile_id TEXT NOT NULL DEFAULT '',
 workspace_id TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 PRIMARY KEY (extension_name, profile_id, workspace_id),
 CHECK (profile_id = trim(profile_id) AND workspace_id = trim(workspace_id))
);

CREATE TRIGGER extension_installations_default_insert
AFTER INSERT ON extensions
BEGIN
 INSERT INTO extension_installations (extension_name, profile_id, workspace_id, created_at)
 VALUES (NEW.name, '', '', NEW.installed_at);
END;

CREATE TRIGGER extension_installations_profile_insert
BEFORE INSERT ON extension_installations
WHEN NEW.profile_id <> '' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id)
BEGIN
 SELECT RAISE(ABORT, 'profile_not_found');
END;

CREATE TRIGGER extension_installations_workspace_insert
BEFORE INSERT ON extension_installations
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
 SELECT RAISE(ABORT, 'workspace_not_found');
END;

CREATE TRIGGER extension_installations_profile_delete
AFTER DELETE ON profiles
BEGIN
 DELETE FROM extension_installations WHERE profile_id = OLD.id;
END;

CREATE TRIGGER extension_installations_workspace_delete
AFTER DELETE ON workspaces
BEGIN
 DELETE FROM extension_installations WHERE workspace_id = OLD.id;
END;

CREATE TRIGGER extension_inputs_workspace_delete
AFTER DELETE ON workspaces
BEGIN
 DELETE FROM extension_inputs WHERE workspace_id = OLD.id;
END;

CREATE TRIGGER extension_inputs_profile_delete
AFTER DELETE ON profiles
BEGIN
 DELETE FROM extension_inputs WHERE profile = OLD.id;
END;

CREATE TRIGGER extension_mcp_overrides_workspace_delete
AFTER DELETE ON workspaces
BEGIN
 DELETE FROM extension_mcp_overrides WHERE workspace_id = OLD.id;
END;

CREATE TRIGGER extension_mcp_overrides_profile_delete
AFTER DELETE ON profiles
BEGIN
 DELETE FROM extension_mcp_overrides WHERE profile = OLD.id;
END;
