-- +goose Up
-- add column "token_endpoint_auth_method" to table: "mcp_oauth_registrations"
ALTER TABLE `mcp_oauth_registrations` ADD COLUMN `token_endpoint_auth_method` text NOT NULL DEFAULT '';
