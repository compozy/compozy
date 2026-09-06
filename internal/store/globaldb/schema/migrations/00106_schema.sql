-- +goose Up
-- add column "deferred_until" to table: "automation_scheduler_state"
ALTER TABLE `automation_scheduler_state` ADD COLUMN `deferred_until` text NULL;
