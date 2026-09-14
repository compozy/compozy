-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_marketplace_catalog_state" table
CREATE TABLE `new_marketplace_catalog_state` (`source` text NOT NULL, `source_ref` text NOT NULL DEFAULT '', `document_digest` text NOT NULL DEFAULT '', `diagnostics_json` text NOT NULL DEFAULT '[]', `manifest_version` integer NOT NULL, `generated_at` text NULL, `fetched_at` text NOT NULL DEFAULT '', `stale` integer NOT NULL DEFAULT 0, `last_error` text NOT NULL DEFAULT '', `kind_of_source` text NOT NULL DEFAULT 'feed', `enabled` integer NOT NULL DEFAULT 1, `plugins` integer NOT NULL DEFAULT 0, `installable` integer NOT NULL DEFAULT 0, `error_class` text NOT NULL DEFAULT '', `document_path` text NOT NULL DEFAULT '', `owner` text NOT NULL DEFAULT '', `revision` text NOT NULL DEFAULT '', `generation` integer NOT NULL DEFAULT 0, PRIMARY KEY (`source`), CHECK (trim(source) <> ''), CHECK (json_valid(diagnostics_json) AND json_type(diagnostics_json) = 'array'), CHECK (manifest_version >= 0), CHECK (stale IN (0, 1)), CHECK (kind_of_source IN ('feed', 'preset', 'custom')), CHECK (enabled IN (0, 1)), CHECK (plugins >= 0), CHECK (installable >= 0), CHECK (generation >= 0));
-- copy rows from old table "marketplace_catalog_state" to new temporary table "new_marketplace_catalog_state"
INSERT INTO `new_marketplace_catalog_state` (`source`, `manifest_version`, `generated_at`, `fetched_at`, `stale`, `last_error`, `kind_of_source`, `enabled`, `plugins`, `installable`, `error_class`, `document_path`, `owner`, `revision`, `generation`) SELECT `source`, `manifest_version`, `generated_at`, `fetched_at`, `stale`, `last_error`, `kind_of_source`, `enabled`, `plugins`, `installable`, `error_class`, `document_path`, `owner`, `revision`, `generation` FROM `marketplace_catalog_state`;
-- drop "marketplace_catalog_state" table after copying rows
DROP TABLE `marketplace_catalog_state`;
-- rename temporary table "new_marketplace_catalog_state" to "marketplace_catalog_state"
ALTER TABLE `new_marketplace_catalog_state` RENAME TO `marketplace_catalog_state`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;

UPDATE marketplace_catalog_state SET source_ref = 'catalog:compozy'
WHERE source = 'compozy-catalog' AND source_ref = '';

-- Retire the former error-class prefix at the migration boundary.
UPDATE marketplace_catalog_state
SET error_class = trim(substr(last_error, 2, instr(last_error, ']') - 2)),
    last_error = trim(substr(last_error, instr(last_error, ']') + 1))
WHERE error_class = '' AND substr(last_error, 1, 1) = '[' AND instr(last_error, ']') > 1;
