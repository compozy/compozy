-- +goose Up
-- add column "attention_producer_node_id" to table: "loop_node_controls"
ALTER TABLE `loop_node_controls` ADD COLUMN `attention_producer_node_id` text NOT NULL DEFAULT '';
