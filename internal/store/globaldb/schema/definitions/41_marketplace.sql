CREATE TABLE marketplace_catalog_entries (
	kind           TEXT NOT NULL CHECK (kind IN ('mcp', 'extension', 'skill')),
	entry_id       TEXT NOT NULL CHECK (trim(entry_id) <> ''),
	name           TEXT NOT NULL CHECK (trim(name) <> ''),
	description    TEXT NOT NULL CHECK (trim(description) <> ''),
	version        TEXT NOT NULL DEFAULT '',
	published_at   TEXT,
	updated_at     TEXT,
	digest_sha256  TEXT,
	tier           TEXT,
	install_slug   TEXT,
	payload_json   TEXT NOT NULL CHECK (json_valid(payload_json)),
	fetched_at     TEXT NOT NULL CHECK (trim(fetched_at) <> ''),
	PRIMARY KEY (kind, entry_id)
);

CREATE INDEX idx_marketplace_catalog_entries_kind_name
	ON marketplace_catalog_entries(kind, name, entry_id);

CREATE TABLE marketplace_catalog_state (
	kind             TEXT NOT NULL PRIMARY KEY CHECK (kind IN ('mcp', 'extension', 'skill')),
	manifest_version INTEGER NOT NULL CHECK (manifest_version >= 0),
	generated_at     TEXT,
	fetched_at       TEXT NOT NULL DEFAULT '',
	stale            INTEGER NOT NULL DEFAULT 0 CHECK (stale IN (0, 1)),
	last_error       TEXT NOT NULL DEFAULT ''
);
