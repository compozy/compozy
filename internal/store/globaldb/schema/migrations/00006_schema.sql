-- +goose Up
-- add column "definition_fingerprint" to table: "mcp_auth_tokens"
ALTER TABLE `mcp_auth_tokens` ADD COLUMN `definition_fingerprint` text NOT NULL DEFAULT '';

