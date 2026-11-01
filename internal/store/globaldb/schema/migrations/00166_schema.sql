-- +goose Up
-- add column "instance_id" to table: "agent_heartbeat_wake_events"
ALTER TABLE `agent_heartbeat_wake_events` ADD COLUMN `instance_id` text NOT NULL DEFAULT '';
