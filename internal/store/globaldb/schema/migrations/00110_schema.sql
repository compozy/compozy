-- +goose Up
-- add column "cache_read_tokens" to table: "token_stats"
ALTER TABLE `token_stats` ADD COLUMN `cache_read_tokens` integer NULL;
-- add column "cache_write_tokens" to table: "token_stats"
ALTER TABLE `token_stats` ADD COLUMN `cache_write_tokens` integer NULL;
