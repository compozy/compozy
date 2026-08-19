-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_run_events" table
CREATE TABLE `new_loop_run_events` (`watch_seq` integer NOT NULL PRIMARY KEY AUTOINCREMENT, `id` text NOT NULL, `loop_run_id` text NOT NULL, `workspace_id` text NOT NULL, `seq` integer NOT NULL, `kind` text NOT NULL, `payload_json` text NOT NULL, `at` timestamp NOT NULL, `delivery_key` text NULL, CHECK (kind IN (
				'node_running','node_succeeded','node_failed','node_quarantined','node_requeued',
				'node_paused','node_resumed','node_wait_started','node_wait_resumed',
				'duplicate_suppressed','node_canceled','node_killed','node_attention_flagged',
				'node_attention_cleared','target_breaker_transition','gate_verdict',
				'generation_started','channel_msg','token_tick','needs_approval','status_changed',
				'goal_turn_started','goal_turn_completed','goal_status_changed','runtime_applied',
				'predicate_diagnostic','route_taken','node_retry_scheduled','stale_schedule_dropped',
				'late_arrival','effect_results','custom_event','request_opened','request_answered',
				'request_expired','request_canceled','node_amended'
			)), CHECK (json_valid(payload_json)));
-- copy rows from old table "loop_run_events" to new temporary table "new_loop_run_events"
INSERT INTO `new_loop_run_events` (`watch_seq`, `id`, `loop_run_id`, `workspace_id`, `seq`, `kind`, `payload_json`, `at`, `delivery_key`) SELECT `watch_seq`, `id`, `loop_run_id`, `workspace_id`, `seq`, `kind`, `payload_json`, `at`, `delivery_key` FROM `loop_run_events`;
-- drop "loop_run_events" table after copying rows
DROP TABLE `loop_run_events`;
-- rename temporary table "new_loop_run_events" to "loop_run_events"
ALTER TABLE `new_loop_run_events` RENAME TO `loop_run_events`;
-- create index "loop_run_events_id" to table: "loop_run_events"
CREATE UNIQUE INDEX `loop_run_events_id` ON `loop_run_events` (`id`);
-- create index "idx_loop_run_events_run_seq" to table: "loop_run_events"
CREATE INDEX `idx_loop_run_events_run_seq` ON `loop_run_events` (`loop_run_id`, `seq`);
-- create index "idx_loop_run_events_watch_stream" to table: "loop_run_events"
CREATE INDEX `idx_loop_run_events_watch_stream` ON `loop_run_events` (`workspace_id`, `watch_seq`);
-- create index "uq_loop_run_events_delivery" to table: "loop_run_events"
CREATE UNIQUE INDEX `uq_loop_run_events_delivery` ON `loop_run_events` (`loop_run_id`, `delivery_key`) WHERE delivery_key IS NOT NULL;
-- create "loop_node_amendments" table
CREATE TABLE `loop_node_amendments` (`workspace_id` text NOT NULL, `loop_run_id` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `amendment_seq` integer NOT NULL, `original_ref` text NOT NULL, `amended_ref` text NOT NULL, `actor_kind` text NOT NULL, `actor_id` text NOT NULL, `reason` text NULL, `created_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `generation`, `node_id`, `item_index`, `amendment_seq`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation >= 1), CHECK (item_index >= 0), CHECK (amendment_seq >= 1));
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
