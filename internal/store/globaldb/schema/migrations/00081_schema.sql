-- +goose Up
ALTER TABLE `loop_generation_outputs` ADD COLUMN `output_id` text NULL;
ALTER TABLE `loop_generation_outputs` ADD COLUMN `artifact_name` text NULL;
