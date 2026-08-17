-- +goose Up
-- add column "notify_creator" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `notify_creator` boolean NOT NULL DEFAULT 1;
