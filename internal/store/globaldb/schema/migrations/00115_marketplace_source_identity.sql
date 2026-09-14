-- +goose Up
-- The removed discriminator is constrained to one value; every catalog row remains.
ALTER TABLE marketplace_catalog_entries DROP COLUMN kind;

-- +goose Down
CREATE TABLE marketplace_catalog_entries_previous (
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

INSERT INTO marketplace_catalog_entries_previous (source, kind, entry_id, name, description, version, published_at, updated_at, digest_sha256, tier, install_slug, payload_json, fetched_at, layout, icon, installable, install_blocker, resolved_ref)
SELECT source, 'extension', entry_id, name, description, version, published_at, updated_at, digest_sha256, tier, install_slug, payload_json, fetched_at, layout, icon, installable, install_blocker, resolved_ref FROM marketplace_catalog_entries;
DROP TABLE marketplace_catalog_entries;
ALTER TABLE marketplace_catalog_entries_previous RENAME TO marketplace_catalog_entries;
CREATE INDEX idx_marketplace_catalog_entries_source_name
 ON marketplace_catalog_entries(source, name, entry_id);
