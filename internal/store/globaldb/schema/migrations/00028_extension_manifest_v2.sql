-- +goose Up
-- Manifest v2 is a greenfield hard cut: unwrap the two persisted v1 objects into
-- closed-set arrays and remove the obsolete column names in one table rebuild.
PRAGMA foreign_keys = off;

CREATE TABLE `new_extensions` (
    `name` text NULL,
    `version` text NOT NULL,
    `source` text NOT NULL,
    `enabled` boolean NOT NULL DEFAULT 1,
    `manifest_path` text NOT NULL,
    `installed_at` text NOT NULL,
    `provides_json` text NOT NULL DEFAULT '[]',
    `permissions_json` text NOT NULL DEFAULT '[]',
    `checksum` text NOT NULL,
    `registry_slug` text NULL,
    `registry_name` text NULL,
    `remote_version` text NULL,
    `provenance_json` text NOT NULL DEFAULT '{}',
    PRIMARY KEY (`name`)
);

INSERT INTO `new_extensions` (
    `name`,
    `version`,
    `source`,
    `enabled`,
    `manifest_path`,
    `installed_at`,
    `provides_json`,
    `permissions_json`,
    `checksum`,
    `registry_slug`,
    `registry_name`,
    `remote_version`,
    `provenance_json`
)
SELECT
    `name`,
    `version`,
    `source`,
    `enabled`,
    `manifest_path`,
    `installed_at`,
    CASE
        WHEN json_valid(`capabilities`)
            THEN COALESCE(json_extract(`capabilities`, '$.provides'), '[]')
        ELSE '[]'
    END,
    CASE
        WHEN json_valid(`actions`)
            THEN COALESCE(json_extract(`actions`, '$.requires'), '[]')
        ELSE '[]'
    END,
    `checksum`,
    `registry_slug`,
    `registry_name`,
    `remote_version`,
    `provenance_json`
FROM `extensions`;

DROP TABLE `extensions`;
ALTER TABLE `new_extensions` RENAME TO `extensions`;

CREATE TABLE `extension_dev_links` (
    `extension_name` text NOT NULL,
    `workspace_id` text NOT NULL,
    `origin_path` text NOT NULL,
    `bundle_generation` text NOT NULL,
    `linked_at` timestamp NOT NULL,
    UNIQUE (`extension_name`, `workspace_id`)
);

PRAGMA foreign_keys = on;
