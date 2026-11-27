-- +goose Up
-- add column "requested_scopes_json" to table: "mcp_auth_tokens"
ALTER TABLE `mcp_auth_tokens` ADD COLUMN `requested_scopes_json` text NOT NULL DEFAULT '[]';
