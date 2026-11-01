-- +goose Up
-- add column "initiator_kind" to table: "permission_log"
ALTER TABLE `permission_log` ADD COLUMN `initiator_kind` text NOT NULL DEFAULT '';
-- add column "initiator_id" to table: "permission_log"
ALTER TABLE `permission_log` ADD COLUMN `initiator_id` text NOT NULL DEFAULT '';
-- add column "initiator_kind" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `initiator_kind` text NOT NULL DEFAULT '';
-- add column "initiator_id" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `initiator_id` text NOT NULL DEFAULT '';
