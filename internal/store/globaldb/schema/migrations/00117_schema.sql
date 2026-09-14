-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_marketplace_catalog_entries" table
CREATE TABLE `new_marketplace_catalog_entries` (`source` text NOT NULL, `entry_id` text NOT NULL, `name` text NOT NULL, `description` text NOT NULL, `version` text NOT NULL DEFAULT '', `published_at` text NULL, `updated_at` text NULL, `digest_sha256` text NULL, `tier` text NULL, `install_slug` text NULL, `payload_json` text NOT NULL, `fetched_at` text NOT NULL, `layout` text NOT NULL DEFAULT '', `icon` text NOT NULL DEFAULT '', `installable` integer NOT NULL DEFAULT 1, `install_blocker` text NOT NULL DEFAULT '', `resolved_ref` text NOT NULL DEFAULT '', PRIMARY KEY (`source`, `entry_id`), CHECK (trim(source) <> ''), CHECK (trim(entry_id) <> ''), CHECK (trim(name) <> ''), CHECK (json_valid(payload_json)), CHECK (trim(fetched_at) <> ''), CHECK (installable IN (0, 1)));
-- copy rows from old table "marketplace_catalog_entries" to new temporary table "new_marketplace_catalog_entries"
INSERT INTO `new_marketplace_catalog_entries` (`source`, `entry_id`, `name`, `description`, `version`, `published_at`, `updated_at`, `digest_sha256`, `tier`, `install_slug`, `payload_json`, `fetched_at`, `layout`, `icon`, `installable`, `install_blocker`, `resolved_ref`) SELECT `source`, `entry_id`, `name`, `description`, `version`, `published_at`, `updated_at`, `digest_sha256`, `tier`, `install_slug`, `payload_json`, `fetched_at`, `layout`, `icon`, `installable`, `install_blocker`, `resolved_ref` FROM `marketplace_catalog_entries`;
-- drop "marketplace_catalog_entries" table after copying rows
DROP TABLE `marketplace_catalog_entries`;
-- rename temporary table "new_marketplace_catalog_entries" to "marketplace_catalog_entries"
ALTER TABLE `new_marketplace_catalog_entries` RENAME TO `marketplace_catalog_entries`;
-- create index "idx_marketplace_catalog_entries_source_name" to table: "marketplace_catalog_entries"
CREATE INDEX `idx_marketplace_catalog_entries_source_name` ON `marketplace_catalog_entries` (`source`, `name`, `entry_id`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
