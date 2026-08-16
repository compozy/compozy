-- +goose Up
-- create "loop_node_lane_pauses" table
CREATE TABLE `loop_node_lane_pauses` (`workspace_id` text NOT NULL, `loop_run_id` text NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL, `actor_kind` text NOT NULL, `actor_id` text NOT NULL, `reason` text NULL, `mode` text NOT NULL, `requested_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `node_id`, `item_index`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (item_index >= 0), CHECK (mode IN ('drain','cancel')));
