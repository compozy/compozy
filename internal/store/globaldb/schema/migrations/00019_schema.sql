-- +goose Up
-- add column "cost_cache_read_per_million" to table: "model_catalog_rows"
ALTER TABLE `model_catalog_rows` ADD COLUMN `cost_cache_read_per_million` real NULL;
-- add column "cost_cache_write_per_million" to table: "model_catalog_rows"
ALTER TABLE `model_catalog_rows` ADD COLUMN `cost_cache_write_per_million` real NULL;
-- add column "cost_reasoning_per_million" to table: "model_catalog_rows"
ALTER TABLE `model_catalog_rows` ADD COLUMN `cost_reasoning_per_million` real NULL;
