-- +goose Up
-- add column "config_revision" to table: "marketplace_catalog_state"
ALTER TABLE `marketplace_catalog_state` ADD COLUMN `config_revision` text NOT NULL DEFAULT '';
-- create "marketplace_catalog_config" table
CREATE TABLE `marketplace_catalog_config` (`id` integer NOT NULL, `generation` integer NOT NULL DEFAULT 0, `revision` text NOT NULL DEFAULT '', PRIMARY KEY (`id`), CHECK (id = 1), CHECK (generation >= 0));
