-- +goose Up
-- add column "config_options_json" to table: "model_catalog_transport_bindings"
ALTER TABLE `model_catalog_transport_bindings` ADD COLUMN `config_options_json` text NOT NULL DEFAULT 'null';
