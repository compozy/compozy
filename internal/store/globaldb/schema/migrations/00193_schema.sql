-- +goose Up
-- add column "access_scope" to table: "tool_approval_grants"
ALTER TABLE `tool_approval_grants` ADD COLUMN `access_scope` text NOT NULL DEFAULT '';
