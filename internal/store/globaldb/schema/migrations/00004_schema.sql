-- +goose Up
-- create "marketplace_catalog_entries" table
CREATE TABLE `marketplace_catalog_entries` (`kind` text NOT NULL, `entry_id` text NOT NULL, `name` text NOT NULL, `description` text NOT NULL, `version` text NOT NULL DEFAULT '', `published_at` text NULL, `updated_at` text NULL, `digest_sha256` text NULL, `tier` text NULL, `install_slug` text NULL, `payload_json` text NOT NULL, `fetched_at` text NOT NULL, PRIMARY KEY (`kind`, `entry_id`), CHECK (kind IN ('mcp', 'extension', 'skill')), CHECK (trim(entry_id) <> ''), CHECK (trim(name) <> ''), CHECK (trim(description) <> ''), CHECK (json_valid(payload_json)), CHECK (trim(fetched_at) <> ''));
-- create index "idx_marketplace_catalog_entries_kind_name" to table: "marketplace_catalog_entries"
CREATE INDEX `idx_marketplace_catalog_entries_kind_name` ON `marketplace_catalog_entries` (`kind`, `name`, `entry_id`);
-- create "marketplace_catalog_state" table
CREATE TABLE `marketplace_catalog_state` (`kind` text NOT NULL, `manifest_version` integer NOT NULL, `generated_at` text NULL, `fetched_at` text NOT NULL DEFAULT '', `stale` integer NOT NULL DEFAULT 0, `last_error` text NOT NULL DEFAULT '', PRIMARY KEY (`kind`), CHECK (kind IN ('mcp', 'extension', 'skill')), CHECK (manifest_version >= 0), CHECK (stale IN (0, 1)));

