CREATE TABLE extensions (
		name          TEXT PRIMARY KEY,
		version       TEXT NOT NULL,
		source        TEXT NOT NULL,
		enabled       BOOLEAN NOT NULL DEFAULT 1,
		manifest_path TEXT NOT NULL,
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
		workspace_id TEXT NOT NULL DEFAULT '',
		env_name TEXT NOT NULL,
		secret_ref TEXT NOT NULL,
		kind TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (extension_name, workspace_id, env_name)
	);

CREATE INDEX idx_extension_env_bindings_secret_ref
	ON extension_env_bindings (secret_ref);
