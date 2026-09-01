-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_session_cleanup" table
CREATE TABLE `new_loop_session_cleanup` (`id` integer NULL PRIMARY KEY AUTOINCREMENT, `cleanup_id` text NOT NULL, `workspace_id` text NOT NULL, `loop_run_id` text NOT NULL, `source_kind` text NOT NULL, `source_id` text NOT NULL, `source_epoch` integer NOT NULL, `session_id` text NOT NULL, `cause` text NOT NULL, `created_at` timestamp NOT NULL, `completed_at` timestamp NULL, CONSTRAINT `0` FOREIGN KEY (`loop_run_id`) REFERENCES `loop_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(cleanup_id)) > 0), CHECK (source_kind IN ('goal-binding','task-run')), CHECK (length(trim(source_id)) > 0), CHECK (source_epoch >= 0), CHECK (length(trim(session_id)) > 0), CHECK (cause IN (
				'terminal','reseed','control-revoked','stop','operator-cancel'
			)), CHECK (completed_at IS NULL OR completed_at >= created_at), CHECK (
				(source_kind = 'goal-binding' AND source_epoch >= 1)
				OR (source_kind = 'task-run' AND source_epoch = 0)
			));
-- copy rows from old table "loop_session_cleanup" to new temporary table "new_loop_session_cleanup"
INSERT INTO `new_loop_session_cleanup` (`id`, `cleanup_id`, `workspace_id`, `loop_run_id`, `source_kind`, `source_id`, `source_epoch`, `session_id`, `cause`, `created_at`, `completed_at`) SELECT `id`, `cleanup_id`, `workspace_id`, `loop_run_id`, `source_kind`, `source_id`, `source_epoch`, `session_id`, `cause`, `created_at`, `completed_at` FROM `loop_session_cleanup`;
-- drop trigger "automation_watch_events_after_insert" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_watch_events_after_insert`;
-- drop trigger "automation_watch_events_after_terminal_update" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `automation_watch_events_after_terminal_update`;
-- drop trigger "loop_runs_profile_owner_active" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `loop_runs_profile_owner_active`;
-- drop trigger "loop_runs_profile_owner_immutable" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `loop_runs_profile_owner_immutable`;
-- drop trigger "workspace_scope_cleanup_after_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `workspace_scope_cleanup_after_delete`;
-- drop "loop_session_cleanup" table after copying rows
DROP TABLE `loop_session_cleanup`;
-- rename temporary table "new_loop_session_cleanup" to "loop_session_cleanup"
ALTER TABLE `new_loop_session_cleanup` RENAME TO `loop_session_cleanup`;
-- create index "loop_session_cleanup_cleanup_id" to table: "loop_session_cleanup"
CREATE UNIQUE INDEX `loop_session_cleanup_cleanup_id` ON `loop_session_cleanup` (`cleanup_id`);
-- create index "loop_session_cleanup_loop_run_id_source_kind_source_id_source_epoch" to table: "loop_session_cleanup"
CREATE UNIQUE INDEX `loop_session_cleanup_loop_run_id_source_kind_source_id_source_epoch` ON `loop_session_cleanup` (`loop_run_id`, `source_kind`, `source_id`, `source_epoch`);
-- create index "idx_loop_session_cleanup_pending" to table: "loop_session_cleanup"
CREATE INDEX `idx_loop_session_cleanup_pending` ON `loop_session_cleanup` (`id`) WHERE completed_at IS NULL;
-- create "new_loop_runs" table
CREATE TABLE `new_loop_runs` (`id` text NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL, `loop_name` text NOT NULL, `status` text NOT NULL, `historical` integer NOT NULL DEFAULT 0, `completion_state` text NOT NULL DEFAULT 'complete', `forked_from_run_id` text NULL, `forked_from_generation` integer NULL, `generation` integer NOT NULL DEFAULT 0, `reattempt_strategy` text NOT NULL DEFAULT 'failed_only', `last_progress_at` timestamp NOT NULL, `completed_at` timestamp NULL, `budget_tokens` integer NOT NULL DEFAULT 0, `budget_wall_sec` integer NOT NULL DEFAULT 0, `budget_on_exceeded` text NOT NULL DEFAULT 'halt', `tokens_used` integer NOT NULL DEFAULT 0, `parent_loop_run_id` text NULL, `pause_requested` integer NOT NULL DEFAULT 0, `inputs_json` text NOT NULL, `created_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `iteration_cap` integer NOT NULL DEFAULT 0, `started_by_kind` text NOT NULL DEFAULT '', `started_by_ref` text NOT NULL DEFAULT '', `started_origin_kind` text NOT NULL DEFAULT '', `started_origin_ref` text NOT NULL DEFAULT '', `started_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `definition_version` integer NOT NULL DEFAULT 0, `definition_digest` text NOT NULL DEFAULT '', `active_gate_id` text NOT NULL DEFAULT '', `active_human_criteria_json` text NOT NULL DEFAULT '[]', `budget_approval_seq` integer NOT NULL DEFAULT 0, `start_metadata_json` text NOT NULL DEFAULT '{}', `origin_kind` text NOT NULL DEFAULT 'catalog', `origin_session_id` text NULL, `goal_cleared_at` timestamp NULL, `budget_version` integer NOT NULL DEFAULT 0, `goal_context_nudge_ratio` real NOT NULL DEFAULT 0.8, `control_actor_kind` text NULL, `control_actor_id` text NULL, `control_requested_at` timestamp NULL, `origin_creation_profile_ref` text NULL, `origin_policy_spec_digest` text NULL, `origin_creation_digest` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `best_generation` integer NULL, `best_score` real NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE RESTRICT, CHECK (historical IN (0, 1)), CHECK (completion_state IN ('complete','partial')), CHECK (budget_version >= 0), CHECK (goal_context_nudge_ratio >= 0.0 AND goal_context_nudge_ratio <= 1.0), CHECK (origin_creation_profile_ref IS NULL OR length(trim(origin_creation_profile_ref)) > 0), CHECK (origin_policy_spec_digest IS NULL OR length(trim(origin_policy_spec_digest)) > 0), CHECK (origin_creation_digest IS NULL OR length(trim(origin_creation_digest)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
						'explicit_request', 'task_profile', 'workspace_coordination',
						'loop_definition', 'automation_job', 'built_in_local'
					)), CHECK (
						(best_generation IS NULL AND best_score IS NULL)
						OR (best_generation IS NOT NULL AND best_score IS NOT NULL
							AND best_generation >= 1 AND best_generation <= generation)
					), CHECK (
						(forked_from_run_id IS NULL AND forked_from_generation IS NULL)
						OR (
							forked_from_run_id IS NOT NULL
							AND forked_from_generation IS NOT NULL
							AND length(trim(forked_from_run_id)) > 0
							AND forked_from_generation >= 1
						)
					));
-- copy rows from old table "loop_runs" to new temporary table "new_loop_runs"
INSERT INTO `new_loop_runs` (`id`, `profile_id`, `workspace_id`, `loop_name`, `status`, `historical`, `completion_state`, `forked_from_run_id`, `forked_from_generation`, `generation`, `reattempt_strategy`, `last_progress_at`, `completed_at`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `best_generation`, `best_score`) SELECT `id`, `profile_id`, `workspace_id`, `loop_name`, `status`, `historical`, `completion_state`, `forked_from_run_id`, `forked_from_generation`, `generation`, `reattempt_strategy`, `last_progress_at`, `completed_at`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `best_generation`, `best_score` FROM `loop_runs`;
-- drop "loop_runs" table after copying rows
DROP TABLE `loop_runs`;
-- rename temporary table "new_loop_runs" to "loop_runs"
ALTER TABLE `new_loop_runs` RENAME TO `loop_runs`;
-- create index "idx_loop_runs_catalog" to table: "loop_runs"
CREATE INDEX `idx_loop_runs_catalog` ON `loop_runs` (`workspace_id`, `loop_name`, `created_at` DESC, `id` DESC, `status`);
-- create index "idx_loop_runs_queue_order" to table: "loop_runs"
CREATE INDEX `idx_loop_runs_queue_order` ON `loop_runs` (`workspace_id`, `loop_name`, `status`, `created_at`, `id`);
-- create index "uq_loop_runs_active_session_goal" to table: "loop_runs"
CREATE UNIQUE INDEX `uq_loop_runs_active_session_goal` ON `loop_runs` (`origin_session_id`) WHERE origin_kind='session'
			  AND historical = 0
			  AND status IN ('queued','running','watching','needs-approval','paused');
-- create "new_loop_run_events" table
CREATE TABLE `new_loop_run_events` (`watch_seq` integer NOT NULL PRIMARY KEY AUTOINCREMENT, `id` text NOT NULL, `loop_run_id` text NOT NULL, `workspace_id` text NOT NULL, `seq` integer NOT NULL, `kind` text NOT NULL, `payload_json` text NOT NULL, `at` timestamp NOT NULL, `delivery_key` text NULL, CHECK (kind IN (
				'node_running','node_succeeded','node_failed','node_quarantined','node_requeued',
				'node_paused','node_resumed','node_wait_started','node_wait_resumed',
				'duplicate_suppressed','node_canceled','node_attention_flagged',
				'node_attention_cleared','target_breaker_transition','gate_verdict',
				'generation_started','channel_msg','token_tick','needs_approval','status_changed',
				'goal_turn_started','goal_turn_completed','goal_status_changed','runtime_applied',
				'predicate_diagnostic','route_taken','node_retry_scheduled','stale_schedule_dropped',
				'late_arrival','effect_results','custom_event','request_opened','request_answered',
				'request_expired','request_canceled','node_amended','branch_pruned','run_forked'
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
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- recreate trigger "automation_watch_events_after_insert" after rebuilding table "automation_runs"
-- +goose StatementBegin
CREATE TRIGGER automation_watch_events_after_insert
AFTER INSERT ON automation_runs
WHEN NEW.status IN ('completed', 'failed')
BEGIN
	INSERT INTO automation_watch_events (
		run_id, job_id, trigger_id, session_id, status, attempt,
		started_at, ended_at, error, agent_name, workspace_id, retry_json
	) VALUES (
		NEW.id,
		COALESCE(NEW.job_id, ''),
		COALESCE(NEW.trigger_id, ''),
		COALESCE(NEW.session_id, ''),
		NEW.status,
		NEW.attempt,
		NEW.started_at,
		NEW.ended_at,
		COALESCE(NEW.error, ''),
		COALESCE(
			(SELECT agent_name FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT agent_name FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		),
		COALESCE(
			NULLIF((SELECT workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			NULLIF((SELECT workspace_id FROM loop_runs WHERE id = NEW.loop_run_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			''
		),
		COALESCE(
			(SELECT retry FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT retry FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		)
	);
END;
-- +goose StatementEnd
-- recreate trigger "automation_watch_events_after_terminal_update" after rebuilding table "automation_runs"
-- +goose StatementBegin
CREATE TRIGGER automation_watch_events_after_terminal_update
AFTER UPDATE OF status ON automation_runs
WHEN NEW.status IN ('completed', 'failed') AND OLD.status NOT IN ('completed', 'failed')
BEGIN
	INSERT INTO automation_watch_events (
		run_id, job_id, trigger_id, session_id, status, attempt,
		started_at, ended_at, error, agent_name, workspace_id, retry_json
	) VALUES (
		NEW.id,
		COALESCE(NEW.job_id, ''),
		COALESCE(NEW.trigger_id, ''),
		COALESCE(NEW.session_id, ''),
		NEW.status,
		NEW.attempt,
		NEW.started_at,
		NEW.ended_at,
		COALESCE(NEW.error, ''),
		COALESCE(
			(SELECT agent_name FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT agent_name FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		),
		COALESCE(
			NULLIF((SELECT workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			NULLIF((SELECT workspace_id FROM loop_runs WHERE id = NEW.loop_run_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			''
		),
		COALESCE(
			(SELECT retry FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT retry FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		)
	);
END;
-- +goose StatementEnd
-- recreate trigger "loop_runs_profile_owner_active" after rebuilding table "loop_runs"
-- +goose StatementBegin
CREATE TRIGGER loop_runs_profile_owner_active BEFORE INSERT ON loop_runs BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- recreate trigger "loop_runs_profile_owner_immutable" after rebuilding table "loop_runs"
-- +goose StatementBegin
CREATE TRIGGER loop_runs_profile_owner_immutable BEFORE UPDATE OF profile_id ON loop_runs
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- recreate trigger "workspace_scope_cleanup_after_delete" after rebuilding table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER workspace_scope_cleanup_after_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM network_wake_events WHERE workspace_id = OLD.id;
	DELETE FROM network_wake_sources WHERE workspace_id = OLD.id;
	DELETE FROM network_message_dispositions WHERE workspace_id = OLD.id;
	DELETE FROM network_live_wakes WHERE workspace_id = OLD.id;
	DELETE FROM network_participation_budgets WHERE workspace_id = OLD.id;
	DELETE FROM network_task_status_projections WHERE workspace_id = OLD.id;
	DELETE FROM network_task_thread_origins WHERE workspace_id = OLD.id;
	DELETE FROM network_thread_session_token_stats WHERE workspace_id = OLD.id;
	DELETE FROM network_thread_participants WHERE workspace_id = OLD.id;
	DELETE FROM network_subscriptions WHERE workspace_id = OLD.id;
	DELETE FROM network_work WHERE workspace_id = OLD.id;
	DELETE FROM network_direct_rooms WHERE workspace_id = OLD.id;
	DELETE FROM network_threads WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_participants WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_kind_counts WHERE workspace_id = OLD.id;
	DELETE FROM network_channel_stats WHERE workspace_id = OLD.id;
	DELETE FROM network_timeline_log WHERE workspace_id = OLD.id;
	DELETE FROM network_channels WHERE workspace_id = OLD.id;
	DELETE FROM network_audit_log WHERE workspace_id = OLD.id;
	DELETE FROM network_coordination_invitations WHERE workspace_id = OLD.id;
	DELETE FROM task_network_coordination WHERE workspace_id = OLD.id;
	DELETE FROM loop_ui_annotations WHERE workspace_id = OLD.id;
	DELETE FROM loop_session_bindings WHERE workspace_id = OLD.id;
	DELETE FROM loop_run_events WHERE workspace_id = OLD.id;
	DELETE FROM loop_runs WHERE workspace_id = OLD.id;
	DELETE FROM loop_goal_session_outbox WHERE workspace_id = OLD.id;
	DELETE FROM loop_session_cleanup WHERE workspace_id = OLD.id;
	DELETE FROM loop_admission_claims WHERE workspace_id = OLD.id;
	DELETE FROM loop_node_lane_pauses WHERE workspace_id = OLD.id;
	DELETE FROM loop_node_amendments WHERE workspace_id = OLD.id;
	DELETE FROM loop_timetravel_ops WHERE workspace_id = OLD.id;
	DELETE FROM loop_requests WHERE workspace_id = OLD.id;
	DELETE FROM loop_gate_decisions WHERE workspace_id = OLD.id;
	DELETE FROM loop_definition_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM loop_config WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_wake_events WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_wake_state WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_revisions WHERE workspace_id = OLD.id;
	DELETE FROM agent_heartbeat_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM agent_soul_revisions WHERE workspace_id = OLD.id;
	DELETE FROM agent_soul_snapshots WHERE workspace_id = OLD.id;
	DELETE FROM session_health WHERE workspace_id = OLD.id;
	DELETE FROM sessions WHERE workspace_id = OLD.id;
	DELETE FROM token_usage_daily WHERE workspace_id = OLD.id;
	DELETE FROM event_summaries WHERE workspace_id = OLD.id;
	DELETE FROM tool_approval_grants WHERE workspace_id = OLD.id;
	DELETE FROM dead_entities WHERE workspace_id = OLD.id;
	DELETE FROM notification_cursors WHERE workspace_id = OLD.id;
	DELETE FROM skill_exposures WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
