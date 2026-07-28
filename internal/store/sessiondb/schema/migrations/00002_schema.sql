-- +goose Up
-- add column "archived" to table: "events"
ALTER TABLE `events` ADD COLUMN `archived` integer NOT NULL DEFAULT 0;

