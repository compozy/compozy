CREATE TABLE marketplace_catalog_entries (
	source         TEXT NOT NULL CHECK (trim(source) <> ''),
	kind           TEXT NOT NULL CHECK (kind = 'extension'),
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
	layout         TEXT NOT NULL DEFAULT '',
 icon           TEXT NOT NULL DEFAULT '',
 installable    INTEGER NOT NULL DEFAULT 1 CHECK (installable IN (0, 1)),
 install_blocker TEXT NOT NULL DEFAULT '',
 resolved_ref   TEXT NOT NULL DEFAULT '',
 PRIMARY KEY (source, entry_id)
);

CREATE INDEX idx_marketplace_catalog_entries_source_name
	ON marketplace_catalog_entries(source, name, entry_id);

CREATE TABLE marketplace_catalog_state (
	source           TEXT NOT NULL PRIMARY KEY CHECK (trim(source) <> ''),
	manifest_version INTEGER NOT NULL CHECK (manifest_version >= 0),
	generated_at     TEXT,
	fetched_at       TEXT NOT NULL DEFAULT '',
	stale            INTEGER NOT NULL DEFAULT 0 CHECK (stale IN (0, 1)),
	last_error       TEXT NOT NULL DEFAULT '',
 kind_of_source   TEXT NOT NULL DEFAULT 'feed' CHECK (kind_of_source IN ('feed', 'preset', 'custom')),
 enabled          INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
 plugins          INTEGER NOT NULL DEFAULT 0 CHECK (plugins >= 0),
 installable      INTEGER NOT NULL DEFAULT 0 CHECK (installable >= 0),
 error_class      TEXT NOT NULL DEFAULT '',
 document_path    TEXT NOT NULL DEFAULT '',
 owner            TEXT NOT NULL DEFAULT '',
 revision         TEXT NOT NULL DEFAULT '',
 generation       INTEGER NOT NULL DEFAULT 0 CHECK (generation >= 0)
);
