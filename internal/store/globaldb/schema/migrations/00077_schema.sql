-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_node_lane_pauses" table
CREATE TABLE `new_loop_node_lane_pauses` (`workspace_id` text NOT NULL, `loop_run_id` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL, `actor_kind` text NOT NULL, `actor_id` text NOT NULL, `reason` text NULL, `mode` text NOT NULL, `requested_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `generation`, `node_id`, `item_index`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation >= 1), CHECK (item_index >= 0), CHECK (mode IN ('drain','cancel')));
-- copy rows from old table "loop_node_lane_pauses" to new temporary table "new_loop_node_lane_pauses"
INSERT INTO `new_loop_node_lane_pauses` (`workspace_id`, `loop_run_id`, `generation`, `node_id`, `item_index`, `actor_kind`, `actor_id`, `reason`, `mode`, `requested_at`) SELECT `pauses`.`workspace_id`, `pauses`.`loop_run_id`, `runs`.`generation`, `pauses`.`node_id`, `pauses`.`item_index`, `pauses`.`actor_kind`, `pauses`.`actor_id`, `pauses`.`reason`, `pauses`.`mode`, `pauses`.`requested_at` FROM `loop_node_lane_pauses` AS `pauses` JOIN `loop_runs` AS `runs` ON `runs`.`id` = `pauses`.`loop_run_id`;
-- drop "loop_node_lane_pauses" table after copying rows
DROP TABLE `loop_node_lane_pauses`;
-- rename temporary table "new_loop_node_lane_pauses" to "loop_node_lane_pauses"
ALTER TABLE `new_loop_node_lane_pauses` RENAME TO `loop_node_lane_pauses`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
