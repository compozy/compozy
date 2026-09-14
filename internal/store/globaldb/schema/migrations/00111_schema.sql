-- +goose Up
-- create "extension_inputs" table
CREATE TABLE `extension_inputs` (`extension` text NOT NULL, `profile` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL DEFAULT '', `input_id` text NOT NULL, `type` text NOT NULL, `value_json` text NOT NULL, `active` integer NOT NULL DEFAULT 1, `updated_at` text NOT NULL, PRIMARY KEY (`extension`, `profile`, `workspace_id`, `input_id`), CHECK (trim(extension) <> ''), CHECK (trim(input_id) <> ''), CHECK (type IN ('string', 'identifier', 'boolean')), CHECK (json_valid(value_json)), CHECK (active IN (0, 1)));
-- create "extension_mcp_overrides" table
CREATE TABLE `extension_mcp_overrides` (`extension` text NOT NULL, `profile` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL DEFAULT '', `server` text NOT NULL, `runtime_name` text NOT NULL, `env_json` text NOT NULL DEFAULT '{}', `headers_json` text NOT NULL DEFAULT '{}', `url` text NOT NULL DEFAULT '', `updated_at` text NOT NULL, PRIMARY KEY (`extension`, `profile`, `workspace_id`, `server`), CHECK (trim(extension) <> ''), CHECK (trim(server) <> ''), CHECK (trim(runtime_name) <> ''), CHECK (json_valid(env_json)), CHECK (json_valid(headers_json)));
-- create index "extension_mcp_overrides_profile_workspace_id_runtime_name" to table: "extension_mcp_overrides"
CREATE UNIQUE INDEX `extension_mcp_overrides_profile_workspace_id_runtime_name` ON `extension_mcp_overrides` (`profile`, `workspace_id`, `runtime_name`);

-- Preserve every extension projection byte; retired cache kinds are discarded under ADR-001.
CREATE TABLE marketplace_catalog_entries_next (
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

INSERT INTO marketplace_catalog_entries_next (source, kind, entry_id, name, description, version, published_at, updated_at, digest_sha256, tier, install_slug, payload_json, fetched_at)
SELECT 'compozy-catalog', kind, entry_id, name, description, version, published_at, updated_at, digest_sha256, tier, install_slug, payload_json, fetched_at FROM marketplace_catalog_entries WHERE kind = 'extension';
DROP TABLE marketplace_catalog_entries;
ALTER TABLE marketplace_catalog_entries_next RENAME TO marketplace_catalog_entries;
CREATE INDEX idx_marketplace_catalog_entries_source_name
	ON marketplace_catalog_entries(source, name, entry_id);

CREATE TABLE marketplace_catalog_state_next (
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

INSERT INTO marketplace_catalog_state_next
(source, manifest_version, generated_at, fetched_at, stale, last_error, plugins, installable)
SELECT 'compozy-catalog', manifest_version, generated_at, fetched_at, stale, last_error,
 (SELECT count(*) FROM marketplace_catalog_entries WHERE source = 'compozy-catalog'),
 (SELECT count(*) FROM marketplace_catalog_entries WHERE source = 'compozy-catalog')
FROM marketplace_catalog_state WHERE kind = 'extension';
DROP TABLE marketplace_catalog_state;
ALTER TABLE marketplace_catalog_state_next RENAME TO marketplace_catalog_state;

-- A slug or URL alone is not acquisition evidence. Other provenance bytes stay untouched.
UPDATE extensions SET provenance_json = json_set(provenance_json,
 '$.source_name', 'compozy-catalog', '$.source_ref', 'catalog:compozy',
 '$.entry_id', json_extract(provenance_json, '$.catalog_entry_id'))
WHERE json_valid(provenance_json)
 AND json_extract(provenance_json, '$.installed_from') = 'marketplace_registry'
 AND json_type(provenance_json, '$.catalog_entry_id') = 'text'
 AND trim(json_extract(provenance_json, '$.catalog_entry_id')) <> '';
