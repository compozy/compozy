-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_node_waits" table
CREATE TABLE `new_loop_node_waits` (`loop_run_id` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `kind` text NOT NULL, `resume_at` timestamp NULL, `next_escalation_at` timestamp NULL, `escalation_cursor` integer NOT NULL DEFAULT 0, `claim_state` text NOT NULL DEFAULT 'waiting', `claimed_by_kind` text NULL, `claimed_by_id` text NULL, `claimed_at` timestamp NULL, `admission_failures` integer NOT NULL DEFAULT 0, `expect_json` text NULL, `ahead_payload_json` text NULL, `issued_epoch` integer NOT NULL DEFAULT 0, `created_at` timestamp NOT NULL, PRIMARY KEY (`loop_run_id`, `generation`, `node_id`, `item_index`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation >= 1), CHECK (kind IN ('timer','event','approval_escalation','request')), CHECK (claim_state IN (
		'waiting','claimed','resumed','intervention_required'
	)), CHECK (expect_json IS NULL OR json_valid(expect_json)), CHECK (ahead_payload_json IS NULL OR json_valid(ahead_payload_json)));
-- copy rows from old table "loop_node_waits" to new temporary table "new_loop_node_waits"
INSERT INTO `new_loop_node_waits` (`loop_run_id`, `generation`, `node_id`, `item_index`, `kind`, `resume_at`, `next_escalation_at`, `escalation_cursor`, `claim_state`, `claimed_by_kind`, `claimed_by_id`, `claimed_at`, `admission_failures`, `expect_json`, `ahead_payload_json`, `issued_epoch`, `created_at`) SELECT `loop_run_id`, `generation`, `node_id`, `item_index`, `kind`, `resume_at`, `next_escalation_at`, `escalation_cursor`, `claim_state`, `claimed_by_kind`, `claimed_by_id`, `claimed_at`, `admission_failures`, `expect_json`, `ahead_payload_json`, `issued_epoch`, `created_at` FROM `loop_node_waits`;
-- drop "loop_node_waits" table after copying rows
DROP TABLE `loop_node_waits`;
-- rename temporary table "new_loop_node_waits" to "loop_node_waits"
ALTER TABLE `new_loop_node_waits` RENAME TO `loop_node_waits`;
-- create index "idx_loop_node_waits_due" to table: "loop_node_waits"
CREATE INDEX `idx_loop_node_waits_due` ON `loop_node_waits` (`resume_at`) WHERE claim_state = 'waiting' AND resume_at IS NOT NULL;
-- create index "idx_loop_node_waits_ladder" to table: "loop_node_waits"
CREATE INDEX `idx_loop_node_waits_ladder` ON `loop_node_waits` (`next_escalation_at`) WHERE claim_state = 'waiting' AND next_escalation_at IS NOT NULL;
-- create index "idx_loop_node_waits_state" to table: "loop_node_waits"
CREATE INDEX `idx_loop_node_waits_state` ON `loop_node_waits` (`claim_state`);
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
				'request_expired','request_canceled'
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
-- create "loop_requests" table
CREATE TABLE `loop_requests` (`workspace_id` text NOT NULL, `loop_run_id` text NOT NULL, `generation` integer NOT NULL, `node_id` text NOT NULL, `item_index` integer NOT NULL DEFAULT 0, `kind` text NOT NULL, `state` text NOT NULL, `prompt` text NOT NULL, `context_preview_json` text NOT NULL DEFAULT '{}', `context_ref` text NULL, `answer_schema_json` text NULL, `edit_schema_json` text NULL, `respond_schema_json` text NULL, `decisions_json` text NOT NULL, `proposed_ref` text NULL, `proposed_preview_json` text NULL, `answered_decision` text NULL, `answered_payload_ref` text NULL, `answered_note` text NULL, `actor_kind` text NULL, `actor_id` text NULL, `opened_at` timestamp NOT NULL, `resolved_at` timestamp NULL, `expires_at` timestamp NULL, PRIMARY KEY (`loop_run_id`, `generation`, `node_id`, `item_index`), CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (generation >= 1), CHECK (item_index >= 0), CHECK (kind IN ('ask','review')), CHECK (state IN ('pending','answered','expired','canceled')), CHECK (json_valid(context_preview_json)), CHECK (answer_schema_json IS NULL OR json_valid(answer_schema_json)), CHECK (edit_schema_json IS NULL OR json_valid(edit_schema_json)), CHECK (respond_schema_json IS NULL OR json_valid(respond_schema_json)), CHECK (json_valid(decisions_json)), CHECK (proposed_preview_json IS NULL OR json_valid(proposed_preview_json)));
-- create index "idx_loop_requests_pending" to table: "loop_requests"
CREATE INDEX `idx_loop_requests_pending` ON `loop_requests` (`workspace_id`, `state`, `expires_at`, `opened_at`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
