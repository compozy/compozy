-- +goose Up
-- add column "rotation_due_session" to table: "agent_instance_wake_state"
ALTER TABLE `agent_instance_wake_state` ADD COLUMN `rotation_due_session` text NOT NULL DEFAULT '';
