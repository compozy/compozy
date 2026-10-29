-- +goose Up
-- add column "archived_at" to table: "workspaces"
ALTER TABLE `workspaces` ADD COLUMN `archived_at` text NULL;
