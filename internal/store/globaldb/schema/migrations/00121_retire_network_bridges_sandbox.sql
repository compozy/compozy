-- +goose Up

-- Retire Network, Bridges, and Sandbox while retaining sessions, tasks, authored context, and cursor projections.

PRAGMA foreign_keys = OFF;

DROP TRIGGER IF EXISTS "agent_heartbeat_revisions_workspace_insert_guard";

DROP TRIGGER IF EXISTS "agent_heartbeat_revisions_workspace_update_guard";

DROP TRIGGER IF EXISTS "agent_heartbeat_snapshots_workspace_insert_guard";

DROP TRIGGER IF EXISTS "agent_heartbeat_snapshots_workspace_update_guard";

DROP TRIGGER IF EXISTS "agent_heartbeat_wake_events_workspace_insert_guard";

DROP TRIGGER IF EXISTS "agent_heartbeat_wake_events_workspace_update_guard";

DROP TRIGGER IF EXISTS "agent_heartbeat_wake_state_workspace_insert_guard";

DROP TRIGGER IF EXISTS "agent_heartbeat_wake_state_workspace_update_guard";

DROP TRIGGER IF EXISTS "agent_soul_revisions_workspace_insert_guard";

DROP TRIGGER IF EXISTS "agent_soul_revisions_workspace_update_guard";

DROP TRIGGER IF EXISTS "agent_soul_snapshots_workspace_insert_guard";

DROP TRIGGER IF EXISTS "agent_soul_snapshots_workspace_update_guard";

DROP TRIGGER IF EXISTS "automation_jobs_profile_owner_active";

DROP TRIGGER IF EXISTS "automation_jobs_profile_owner_immutable";

DROP TRIGGER IF EXISTS "automation_triggers_profile_owner_active";

DROP TRIGGER IF EXISTS "automation_triggers_profile_owner_immutable";

DROP TRIGGER IF EXISTS "automation_watch_events_after_insert";

DROP TRIGGER IF EXISTS "automation_watch_events_after_terminal_update";

DROP TRIGGER IF EXISTS "bridge_instances_profile_owner_active";

DROP TRIGGER IF EXISTS "bridge_instances_profile_owner_immutable";

DROP TRIGGER IF EXISTS "cmd_palette_workspace_delete";

DROP TRIGGER IF EXISTS "dead_entities_profile_owner_active";

DROP TRIGGER IF EXISTS "dead_entities_profile_owner_immutable";

DROP TRIGGER IF EXISTS "dead_entities_workspace_insert_guard";

DROP TRIGGER IF EXISTS "dead_entities_workspace_update_guard";

DROP TRIGGER IF EXISTS "extension_dev_links_profile_enablement_delete";

DROP TRIGGER IF EXISTS "extension_dev_links_profile_enablement_insert";

DROP TRIGGER IF EXISTS "extension_env_bindings_workspace_delete";

DROP TRIGGER IF EXISTS "extension_inputs_workspace_delete";

DROP TRIGGER IF EXISTS "extension_installations_default_insert";

DROP TRIGGER IF EXISTS "extension_installations_workspace_delete";

DROP TRIGGER IF EXISTS "extension_installations_workspace_insert";

DROP TRIGGER IF EXISTS "extension_mcp_overrides_workspace_delete";

DROP TRIGGER IF EXISTS "extensions_profile_enablement_delete";

DROP TRIGGER IF EXISTS "gateway_ingress_bridge_resource_identity_update";

DROP TRIGGER IF EXISTS "gateway_ingress_resource_delete";

DROP TRIGGER IF EXISTS "gateway_ingress_trigger_resource_identity_update";

DROP TRIGGER IF EXISTS "loop_runs_profile_owner_active";

DROP TRIGGER IF EXISTS "loop_runs_profile_owner_immutable";

DROP TRIGGER IF EXISTS "network_channels_profile_owner_active";

DROP TRIGGER IF EXISTS "network_channels_profile_owner_immutable";

DROP TRIGGER IF EXISTS "network_channels_workspace_insert_guard";

DROP TRIGGER IF EXISTS "network_channels_workspace_update_guard";

DROP TRIGGER IF EXISTS "network_coordination_invitations_workspace_insert_guard";

DROP TRIGGER IF EXISTS "network_coordination_invitations_workspace_update_guard";

DROP TRIGGER IF EXISTS "network_direct_rooms_profile_owner_active";

DROP TRIGGER IF EXISTS "network_direct_rooms_profile_owner_immutable";

DROP TRIGGER IF EXISTS "network_live_wakes_workspace_insert_guard";

DROP TRIGGER IF EXISTS "network_live_wakes_workspace_update_guard";

DROP TRIGGER IF EXISTS "network_message_dispositions_workspace_insert_guard";

DROP TRIGGER IF EXISTS "network_message_dispositions_workspace_update_guard";

DROP TRIGGER IF EXISTS "network_participation_budgets_workspace_insert_guard";

DROP TRIGGER IF EXISTS "network_participation_budgets_workspace_update_guard";

DROP TRIGGER IF EXISTS "network_threads_profile_owner_active";

DROP TRIGGER IF EXISTS "network_threads_profile_owner_immutable";

DROP TRIGGER IF EXISTS "network_wake_events_workspace_insert_guard";

DROP TRIGGER IF EXISTS "network_wake_events_workspace_update_guard";

DROP TRIGGER IF EXISTS "network_wake_sources_workspace_insert_guard";

DROP TRIGGER IF EXISTS "network_wake_sources_workspace_update_guard";

DROP TRIGGER IF EXISTS "network_work_profile_owner_active";

DROP TRIGGER IF EXISTS "network_work_profile_owner_immutable";

DROP TRIGGER IF EXISTS "profile_selections_workspace_delete";

DROP TRIGGER IF EXISTS "session_health_workspace_insert_guard";

DROP TRIGGER IF EXISTS "session_health_workspace_update_guard";

DROP TRIGGER IF EXISTS "sessions_profile_owner_active";

DROP TRIGGER IF EXISTS "sessions_profile_owner_immutable";

DROP TRIGGER IF EXISTS "sessions_workspace_insert_guard";

DROP TRIGGER IF EXISTS "sessions_workspace_update_guard";

DROP TRIGGER IF EXISTS "task_network_coordination_workspace_insert_guard";

DROP TRIGGER IF EXISTS "task_network_coordination_workspace_update_guard";

DROP TRIGGER IF EXISTS "tasks_profile_owner_active";

DROP TRIGGER IF EXISTS "tasks_profile_owner_immutable";

DROP TRIGGER IF EXISTS "tool_approval_grants_workspace_insert_guard";

DROP TRIGGER IF EXISTS "tool_approval_grants_workspace_update_guard";

DROP TRIGGER IF EXISTS "trg_bridge_instance_active_delivery_delete";

DROP TRIGGER IF EXISTS "trg_bridge_instance_active_delivery_identity";

DROP TRIGGER IF EXISTS "trg_sessions_archive_insert_guard";

DROP TRIGGER IF EXISTS "trg_sessions_archive_update_guard";

DROP TRIGGER IF EXISTS "trg_task_runs_terminal_command_delete_guard";

DROP TRIGGER IF EXISTS "trg_task_runs_terminal_command_guard";

DROP TRIGGER IF EXISTS "trg_tasks_terminal_command_delete_guard";

DROP TRIGGER IF EXISTS "workspace_deletion_intents_registration_guard";

DROP TRIGGER IF EXISTS "workspace_scope_cleanup_after_delete";

DELETE FROM "task_run_idempotency" WHERE run_id IN (SELECT id FROM task_runs WHERE run_kind = 'network_wake');

DELETE FROM "task_run_preferred_capabilities" WHERE run_id IN (SELECT id FROM task_runs WHERE run_kind = 'network_wake');

DELETE FROM "task_run_required_capabilities" WHERE run_id IN (SELECT id FROM task_runs WHERE run_kind = 'network_wake');

DELETE FROM "task_run_reviews" WHERE run_id IN (SELECT id FROM task_runs WHERE run_kind = 'network_wake');

DELETE FROM "task_run_starvation" WHERE run_id IN (SELECT id FROM task_runs WHERE run_kind = 'network_wake');

DELETE FROM "task_run_terminal_commands" WHERE run_id IN (SELECT id FROM task_runs WHERE run_kind = 'network_wake');

UPDATE "task_events" SET "run_id" = NULL WHERE "run_id" IN (SELECT id FROM task_runs WHERE run_kind = 'network_wake');

UPDATE "tasks" SET "current_run_id" = NULL WHERE "current_run_id" IN (SELECT id FROM task_runs WHERE run_kind = 'network_wake');

UPDATE "task_runs" SET "parent_run_id" = NULL WHERE "parent_run_id" IN (SELECT id FROM task_runs WHERE run_kind = 'network_wake');

DELETE FROM vault_secrets WHERE ref GLOB 'vault:bridges/*' OR ref GLOB 'vault:sandbox/*';

DELETE FROM resource_records WHERE kind = 'bridge.instance';

DELETE FROM loop_run_events WHERE kind = 'channel_msg';

DELETE FROM task_runs WHERE run_kind = 'network_wake';

DELETE FROM gateway_ingress_bindings WHERE subject_kind = 'bridge_instance';

DELETE FROM dead_entities WHERE kind = 'bridge';

UPDATE "tasks" SET "created_by_kind" = 'daemon' WHERE "created_by_kind" = 'network_peer';

UPDATE "tasks" SET origin_kind = 'daemon', origin_ref = 'retired-network:' || origin_ref WHERE origin_kind = 'network';

UPDATE "task_events" SET "actor_kind" = 'daemon' WHERE "actor_kind" = 'network_peer';

UPDATE "task_events" SET origin_kind = 'daemon', origin_ref = 'retired-network:' || origin_ref WHERE origin_kind = 'network';

DELETE FROM "task_triage_state" WHERE "actor_kind" = 'network_peer';

UPDATE "task_runs" SET "claimed_by_kind" = 'daemon' WHERE "claimed_by_kind" = 'network_peer';

UPDATE "task_runs" SET "terminalized_by_actor_kind" = 'daemon' WHERE "terminalized_by_actor_kind" = 'network_peer';

UPDATE "task_runs" SET origin_kind = 'daemon', origin_ref = 'retired-network:' || origin_ref WHERE origin_kind = 'network';

UPDATE tasks SET owner_kind = NULL, owner_ref = NULL WHERE owner_kind = 'network_peer';

UPDATE task_run_idempotency SET origin_kind = 'daemon', origin_ref = 'retired-network:' || origin_ref WHERE origin_kind = 'network';

UPDATE task_run_reviews SET reviewed_by_kind = 'daemon' WHERE reviewed_by_kind = 'network_peer';

CREATE TABLE "new_automation_jobs" (
		id           TEXT PRIMARY KEY,
		profile_id   TEXT NOT NULL REFERENCES profiles(id),
		scope        TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
		name         TEXT NOT NULL,
		agent_name   TEXT NOT NULL,
		workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		prompt       TEXT NOT NULL,
		schedule     TEXT,
		task         TEXT,
		enabled      BOOLEAN NOT NULL DEFAULT 1,
		retry        TEXT NOT NULL,
		fire_limit   TEXT NOT NULL,
		source       TEXT NOT NULL DEFAULT 'dynamic',
		target_kind  TEXT NOT NULL DEFAULT 'agent' CHECK (target_kind IN ('agent', 'loop')),
		loop_workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		loop_name    TEXT,
		loop_inputs  TEXT,
		loop_input_mapping TEXT,
		created_at   TEXT NOT NULL,
		updated_at   TEXT NOT NULL,
		CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		)
	);

INSERT INTO "new_automation_jobs" ("id", "profile_id", "scope", "name", "agent_name", "workspace_id", "prompt", "schedule", "task", "enabled", "retry", "fire_limit", "source", "target_kind", "loop_workspace_id", "loop_name", "loop_inputs", "loop_input_mapping", "created_at", "updated_at") SELECT "id", "profile_id", "scope", "name", "agent_name", "workspace_id", "prompt", "schedule", "task", "enabled", "retry", "fire_limit", "source", "target_kind", "loop_workspace_id", "loop_name", "loop_inputs", "loop_input_mapping", "created_at", "updated_at" FROM "automation_jobs";

DROP TABLE "automation_jobs";

ALTER TABLE "new_automation_jobs" RENAME TO "automation_jobs";

CREATE INDEX idx_automation_jobs_enabled ON automation_jobs(enabled);

CREATE INDEX idx_automation_jobs_loop_target
			ON automation_jobs(loop_name, loop_workspace_id) WHERE target_kind = 'loop';

CREATE INDEX idx_automation_jobs_profile ON automation_jobs(profile_id, id);

CREATE UNIQUE INDEX uq_automation_jobs_global_name ON automation_jobs(name) WHERE scope = 'global';

CREATE UNIQUE INDEX uq_automation_jobs_workspace_name ON automation_jobs(workspace_id, name) WHERE scope = 'workspace';

CREATE TABLE "new_automation_runs" (
		id         TEXT PRIMARY KEY,
		profile_id TEXT REFERENCES profiles(id),
		job_id     TEXT,
		trigger_id TEXT,
		session_id TEXT,
		task_id    TEXT,
		task_run_id TEXT,
		status     TEXT NOT NULL,
		attempt    INTEGER NOT NULL DEFAULT 1,
		started_at TEXT,
		ended_at   TEXT,
		error      TEXT,
		loop_run_id TEXT REFERENCES loop_runs(id) ON DELETE SET NULL
	, fire_id TEXT, scheduled_at TEXT, delivery_error TEXT, delivery_error_at TEXT, metadata_json TEXT NOT NULL DEFAULT '{}');

INSERT INTO "new_automation_runs" ("id", "profile_id", "job_id", "trigger_id", "session_id", "task_id", "task_run_id", "status", "attempt", "started_at", "ended_at", "error", "loop_run_id", "fire_id", "scheduled_at", "delivery_error", "delivery_error_at", "metadata_json") SELECT "id", "profile_id", "job_id", "trigger_id", "session_id", "task_id", "task_run_id", "status", "attempt", "started_at", "ended_at", "error", "loop_run_id", "fire_id", "scheduled_at", "delivery_error", "delivery_error_at", "metadata_json" FROM "automation_runs";

DROP TABLE "automation_runs";

ALTER TABLE "new_automation_runs" RENAME TO "automation_runs";

CREATE INDEX idx_automation_runs_job ON automation_runs(job_id);

CREATE INDEX idx_automation_runs_loop_run ON automation_runs(loop_run_id);

CREATE INDEX idx_automation_runs_started ON automation_runs(started_at);

CREATE INDEX idx_automation_runs_status ON automation_runs(status);

CREATE INDEX idx_automation_runs_trigger ON automation_runs(trigger_id);

CREATE UNIQUE INDEX uq_automation_runs_fire_id
			ON automation_runs(fire_id) WHERE fire_id IS NOT NULL;

CREATE TABLE "new_automation_triggers" (
		id            TEXT PRIMARY KEY,
		profile_id    TEXT NOT NULL REFERENCES profiles(id),
		scope         TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
		name          TEXT NOT NULL,
		agent_name    TEXT NOT NULL,
		workspace_id  TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		prompt        TEXT NOT NULL,
		event         TEXT NOT NULL,
		filter        TEXT,
		enabled       BOOLEAN NOT NULL DEFAULT 1,
		retry         TEXT NOT NULL,
		fire_limit    TEXT NOT NULL,
		source        TEXT NOT NULL DEFAULT 'dynamic',
		webhook_id    TEXT,
		endpoint_slug TEXT,
		webhook_secret_ref TEXT,
		target_kind   TEXT NOT NULL DEFAULT 'agent' CHECK (target_kind IN ('agent', 'loop')),
		loop_workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		loop_name     TEXT,
		loop_inputs   TEXT,
		loop_input_mapping TEXT,
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL,
		CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		)
	);

INSERT INTO "new_automation_triggers" ("id", "profile_id", "scope", "name", "agent_name", "workspace_id", "prompt", "event", "filter", "enabled", "retry", "fire_limit", "source", "webhook_id", "endpoint_slug", "webhook_secret_ref", "target_kind", "loop_workspace_id", "loop_name", "loop_inputs", "loop_input_mapping", "created_at", "updated_at") SELECT "id", "profile_id", "scope", "name", "agent_name", "workspace_id", "prompt", "event", "filter", "enabled", "retry", "fire_limit", "source", "webhook_id", "endpoint_slug", "webhook_secret_ref", "target_kind", "loop_workspace_id", "loop_name", "loop_inputs", "loop_input_mapping", "created_at", "updated_at" FROM "automation_triggers";

DROP TABLE "automation_triggers";

ALTER TABLE "new_automation_triggers" RENAME TO "automation_triggers";

CREATE INDEX idx_automation_triggers_enabled ON automation_triggers(enabled);

CREATE INDEX idx_automation_triggers_event ON automation_triggers(event);

CREATE INDEX idx_automation_triggers_loop_target
			ON automation_triggers(loop_name, loop_workspace_id) WHERE target_kind = 'loop';

CREATE INDEX idx_automation_triggers_profile ON automation_triggers(profile_id, id);

CREATE UNIQUE INDEX uq_automation_triggers_global_name ON automation_triggers(name) WHERE scope = 'global';

CREATE UNIQUE INDEX uq_automation_triggers_webhook_id ON automation_triggers(webhook_id) WHERE webhook_id IS NOT NULL;

CREATE UNIQUE INDEX uq_automation_triggers_workspace_name ON automation_triggers(workspace_id, name) WHERE scope = 'workspace';

CREATE TABLE "new_dead_entities" (
	profile_id   TEXT NOT NULL REFERENCES profiles(id),
	workspace_id TEXT NOT NULL,
	kind         TEXT NOT NULL CHECK (kind IN ('extension', 'mcp_sidecar', 'loop_target')),
	entity_id    TEXT NOT NULL CHECK (trim(entity_id) <> ''),
	reason       TEXT NOT NULL CHECK (trim(reason) <> ''),
	marked_at    TEXT NOT NULL,
	PRIMARY KEY (profile_id, workspace_id, kind, entity_id)
);

INSERT INTO "new_dead_entities" ("profile_id", "workspace_id", "kind", "entity_id", "reason", "marked_at") SELECT "profile_id", "workspace_id", "kind", "entity_id", "reason", "marked_at" FROM "dead_entities";

DROP TABLE "dead_entities";

ALTER TABLE "new_dead_entities" RENAME TO "dead_entities";

CREATE TABLE "new_extension_dev_links" (
		extension_name TEXT NOT NULL,
		workspace_id TEXT NOT NULL,
		origin_path TEXT NOT NULL,
		bundle_generation TEXT NOT NULL,
		linked_at TIMESTAMP NOT NULL,
		format TEXT NOT NULL DEFAULT 'compozy',
		ingest_diagnostics_json TEXT NOT NULL DEFAULT '[]',
		gateway_requirement_digest TEXT NOT NULL DEFAULT '',
		gateway_confirmed_by TEXT,
		gateway_confirmed_at TEXT,
		UNIQUE (extension_name, workspace_id)
	);

INSERT INTO "new_extension_dev_links" ("extension_name", "workspace_id", "origin_path", "bundle_generation", "linked_at", "format", "ingest_diagnostics_json", "gateway_requirement_digest", "gateway_confirmed_by", "gateway_confirmed_at") SELECT "extension_name", "workspace_id", "origin_path", "bundle_generation", "linked_at", "format", "ingest_diagnostics_json", "network_requirement_digest", "network_confirmed_by", "network_confirmed_at" FROM "extension_dev_links";

DROP TABLE "extension_dev_links";

ALTER TABLE "new_extension_dev_links" RENAME TO "extension_dev_links";

CREATE TABLE "new_extensions" (
		name          TEXT PRIMARY KEY,
		version       TEXT NOT NULL,
		source        TEXT NOT NULL,
		manifest_path TEXT NOT NULL,
		format        TEXT NOT NULL DEFAULT 'compozy',
		ingest_diagnostics_json TEXT NOT NULL DEFAULT '[]',
		installed_at  TEXT NOT NULL,
		provides_json TEXT NOT NULL DEFAULT '[]',
		permissions_json TEXT NOT NULL DEFAULT '[]',
		checksum      TEXT NOT NULL,
		lifecycle_token TEXT NOT NULL DEFAULT '',
		registry_slug TEXT,
		registry_name TEXT,
		remote_version TEXT,
		provenance_json TEXT NOT NULL DEFAULT '{}',
		gateway_requirement_digest TEXT NOT NULL DEFAULT '',
		gateway_confirmed_by TEXT,
		gateway_confirmed_at TEXT
	);

INSERT INTO "new_extensions" ("name", "version", "source", "manifest_path", "format", "ingest_diagnostics_json", "installed_at", "provides_json", "permissions_json", "checksum", "lifecycle_token", "registry_slug", "registry_name", "remote_version", "provenance_json", "gateway_requirement_digest", "gateway_confirmed_by", "gateway_confirmed_at") SELECT "name", "version", "source", "manifest_path", "format", "ingest_diagnostics_json", "installed_at", "provides_json", "permissions_json", "checksum", "lifecycle_token", "registry_slug", "registry_name", "remote_version", "provenance_json", "network_requirement_digest", "network_confirmed_by", "network_confirmed_at" FROM "extensions";

DROP TABLE "extensions";

ALTER TABLE "new_extensions" RENAME TO "extensions";

CREATE TABLE "new_gateway_ingress_bindings" (
	subject_kind TEXT NOT NULL CHECK (subject_kind IN ('webhook_trigger')),
	subject_id TEXT NOT NULL CHECK (length(trim(subject_id)) > 0),
	scope_kind TEXT NOT NULL CHECK (scope_kind IN ('global', 'workspace')),
	workspace_id TEXT,
	endpoint_generation INTEGER NOT NULL CHECK (endpoint_generation > 0),
	confirmed_at TEXT NOT NULL,
	PRIMARY KEY (subject_kind, subject_id),
	CHECK (
		(scope_kind = 'global' AND workspace_id IS NULL) OR
		(scope_kind = 'workspace' AND workspace_id IS NOT NULL)
	)
);

INSERT INTO "new_gateway_ingress_bindings" ("subject_kind", "subject_id", "scope_kind", "workspace_id", "endpoint_generation", "confirmed_at") SELECT "subject_kind", "subject_id", "scope_kind", "workspace_id", "endpoint_generation", "confirmed_at" FROM "gateway_ingress_bindings";

DROP TABLE "gateway_ingress_bindings";

ALTER TABLE "new_gateway_ingress_bindings" RENAME TO "gateway_ingress_bindings";

CREATE INDEX gateway_ingress_bindings_workspace
	ON gateway_ingress_bindings(workspace_id, subject_kind, subject_id)
	WHERE workspace_id IS NOT NULL;

CREATE TABLE "new_loop_run_events" (
			watch_seq    INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
			id           TEXT NOT NULL UNIQUE,
			loop_run_id  TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			seq          INTEGER NOT NULL,
			kind         TEXT NOT NULL CHECK (kind IN (
				'node_running','node_succeeded','node_failed','node_quarantined','node_requeued',
				'node_paused','node_resumed','node_wait_started','node_wait_resumed',
				'duplicate_suppressed','node_canceled','node_attention_flagged',
				'node_attention_cleared','target_breaker_transition','gate_verdict',
				'generation_started','token_tick','needs_approval','status_changed',
				'goal_turn_started','goal_turn_completed','goal_status_changed','runtime_applied',
				'predicate_diagnostic','route_taken','node_retry_scheduled','stale_schedule_dropped',
				'late_arrival','effect_results','custom_event','request_opened','request_answered',
				'request_expired','request_canceled','node_amended','branch_pruned','run_forked'
			)),
			payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
			at           TIMESTAMP NOT NULL,
			delivery_key TEXT
		);

INSERT INTO "new_loop_run_events" ("watch_seq", "id", "loop_run_id", "workspace_id", "seq", "kind", "payload_json", "at", "delivery_key") SELECT "watch_seq", "id", "loop_run_id", "workspace_id", "seq", "kind", "payload_json", "at", "delivery_key" FROM "loop_run_events";

DROP TABLE "loop_run_events";

ALTER TABLE "new_loop_run_events" RENAME TO "loop_run_events";

CREATE INDEX idx_loop_run_events_run_seq
			ON loop_run_events(loop_run_id, seq);

CREATE INDEX idx_loop_run_events_watch_stream
	ON loop_run_events(workspace_id, watch_seq);

CREATE UNIQUE INDEX uq_loop_run_events_delivery
	ON loop_run_events(loop_run_id, delivery_key) WHERE delivery_key IS NOT NULL;

CREATE TABLE "new_loop_runs" (
			id                   TEXT PRIMARY KEY,
			profile_id           TEXT NOT NULL REFERENCES profiles(id) ON DELETE RESTRICT,
			workspace_id         TEXT NOT NULL,
			loop_name            TEXT NOT NULL,
			status               TEXT NOT NULL,
			historical           INTEGER NOT NULL DEFAULT 0 CHECK (historical IN (0, 1)),
			completion_state     TEXT NOT NULL DEFAULT 'complete'
				CHECK (completion_state IN ('complete','partial')),
			forked_from_run_id   TEXT,
			forked_from_generation INTEGER,
			generation           INTEGER NOT NULL DEFAULT 0,
			reattempt_strategy   TEXT NOT NULL DEFAULT 'failed_only',
			last_progress_at     TIMESTAMP NOT NULL,
			completed_at         TIMESTAMP,
			budget_tokens        INTEGER NOT NULL DEFAULT 0,
			budget_wall_sec      INTEGER NOT NULL DEFAULT 0,
			budget_on_exceeded   TEXT NOT NULL DEFAULT 'halt',
			tokens_used          INTEGER NOT NULL DEFAULT 0,
			parent_loop_run_id   TEXT,
			pause_requested      INTEGER NOT NULL DEFAULT 0,
			inputs_json          TEXT NOT NULL
		, created_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', iteration_cap INTEGER NOT NULL DEFAULT 0, started_by_kind TEXT NOT NULL DEFAULT '', started_by_ref TEXT NOT NULL DEFAULT '', started_origin_kind TEXT NOT NULL DEFAULT '', started_origin_ref TEXT NOT NULL DEFAULT '', started_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', definition_version INTEGER NOT NULL DEFAULT 0, definition_digest TEXT NOT NULL DEFAULT '', active_gate_id TEXT NOT NULL DEFAULT '', active_human_criteria_json TEXT NOT NULL DEFAULT '[]', budget_approval_seq INTEGER NOT NULL DEFAULT 0, start_metadata_json TEXT NOT NULL DEFAULT '{}', origin_kind TEXT NOT NULL DEFAULT 'catalog', origin_session_id TEXT, goal_cleared_at TIMESTAMP, budget_version INTEGER NOT NULL DEFAULT 0 CHECK (budget_version >= 0), goal_context_nudge_ratio REAL NOT NULL DEFAULT 0.8
				CHECK (goal_context_nudge_ratio >= 0.0 AND goal_context_nudge_ratio <= 1.0), control_actor_kind TEXT, control_actor_id TEXT, control_requested_at TIMESTAMP, origin_creation_profile_ref TEXT
				CHECK (origin_creation_profile_ref IS NULL OR length(trim(origin_creation_profile_ref)) > 0), origin_policy_spec_digest TEXT
				CHECK (origin_policy_spec_digest IS NULL OR length(trim(origin_policy_spec_digest)) > 0), origin_creation_digest TEXT
				CHECK (origin_creation_digest IS NULL OR length(trim(origin_creation_digest)) > 0), best_generation INTEGER, best_score REAL,
					CHECK (
						(best_generation IS NULL AND best_score IS NULL)
						OR (best_generation IS NOT NULL AND best_score IS NOT NULL
							AND best_generation >= 1 AND best_generation <= generation)
					),
					CHECK (
						(forked_from_run_id IS NULL AND forked_from_generation IS NULL)
						OR (
							forked_from_run_id IS NOT NULL
							AND forked_from_generation IS NOT NULL
							AND length(trim(forked_from_run_id)) > 0
							AND forked_from_generation >= 1
						)
					));

INSERT INTO "new_loop_runs" ("id", "profile_id", "workspace_id", "loop_name", "status", "historical", "completion_state", "forked_from_run_id", "forked_from_generation", "generation", "reattempt_strategy", "last_progress_at", "completed_at", "budget_tokens", "budget_wall_sec", "budget_on_exceeded", "tokens_used", "parent_loop_run_id", "pause_requested", "inputs_json", "created_at", "iteration_cap", "started_by_kind", "started_by_ref", "started_origin_kind", "started_origin_ref", "started_at", "definition_version", "definition_digest", "active_gate_id", "active_human_criteria_json", "budget_approval_seq", "start_metadata_json", "origin_kind", "origin_session_id", "goal_cleared_at", "budget_version", "goal_context_nudge_ratio", "control_actor_kind", "control_actor_id", "control_requested_at", "origin_creation_profile_ref", "origin_policy_spec_digest", "origin_creation_digest", "best_generation", "best_score") SELECT "id", "profile_id", "workspace_id", "loop_name", "status", "historical", "completion_state", "forked_from_run_id", "forked_from_generation", "generation", "reattempt_strategy", "last_progress_at", "completed_at", "budget_tokens", "budget_wall_sec", "budget_on_exceeded", "tokens_used", "parent_loop_run_id", "pause_requested", "inputs_json", "created_at", "iteration_cap", "started_by_kind", "started_by_ref", "started_origin_kind", "started_origin_ref", "started_at", "definition_version", "definition_digest", "active_gate_id", "active_human_criteria_json", "budget_approval_seq", "start_metadata_json", "origin_kind", "origin_session_id", "goal_cleared_at", "budget_version", "goal_context_nudge_ratio", "control_actor_kind", "control_actor_id", "control_requested_at", "origin_creation_profile_ref", "origin_policy_spec_digest", "origin_creation_digest", "best_generation", "best_score" FROM "loop_runs";

DROP TABLE "loop_runs";

ALTER TABLE "new_loop_runs" RENAME TO "loop_runs";

CREATE INDEX idx_loop_runs_catalog
			ON loop_runs(workspace_id, loop_name, created_at DESC, id DESC, status);

CREATE INDEX idx_loop_runs_queue_order
			ON loop_runs(workspace_id, loop_name, status, created_at ASC, id ASC);

CREATE UNIQUE INDEX uq_loop_runs_active_session_goal ON loop_runs(origin_session_id)
			WHERE origin_kind='session'
			  AND historical = 0
			  AND status IN ('queued','running','watching','needs-approval','paused');

CREATE TABLE "new_profile_lifecycle_op_seed" (
	op_id TEXT PRIMARY KEY REFERENCES profile_lifecycle_ops(id) ON DELETE CASCADE,
	color TEXT NOT NULL CHECK (trim(color) <> ''),
	icon TEXT,
	emoji TEXT,
	default_agent TEXT,
	default_provider TEXT,
	declaration_digest TEXT NOT NULL CHECK (trim(declaration_digest) <> ''),
	CHECK ((icon IS NULL) <> (emoji IS NULL))
);

INSERT INTO "new_profile_lifecycle_op_seed" ("op_id", "color", "icon", "emoji", "default_agent", "default_provider", "declaration_digest") SELECT "op_id", "color", "icon", "emoji", "default_agent", "default_provider", "declaration_digest" FROM "profile_lifecycle_op_seed";

DROP TABLE "profile_lifecycle_op_seed";

ALTER TABLE "new_profile_lifecycle_op_seed" RENAME TO "profile_lifecycle_op_seed";

CREATE TABLE "new_sessions" (
		id             TEXT PRIMARY KEY,
		profile_id     TEXT NOT NULL REFERENCES profiles(id),
		name           TEXT,
		agent_name     TEXT NOT NULL,
		provider       TEXT NOT NULL DEFAULT '',
		model          TEXT NOT NULL DEFAULT '',
		reasoning_effort TEXT NOT NULL DEFAULT '',
		speed          TEXT NOT NULL DEFAULT '',
		acp_options_json TEXT NOT NULL DEFAULT '[]'
			CHECK (json_valid(acp_options_json)),
		speed_resolution_json TEXT NOT NULL DEFAULT '',
		runtime_status TEXT NOT NULL DEFAULT 'unbound',
		runtime_transition TEXT NOT NULL DEFAULT '',
		runtime_failure TEXT NOT NULL DEFAULT '',
		runtime_generation INTEGER NOT NULL DEFAULT 0 CHECK (runtime_generation >= 0),
		runtime_recovery_json TEXT NOT NULL DEFAULT ''
			CHECK (runtime_recovery_json = '' OR json_valid(runtime_recovery_json)),
		selected_provider TEXT NOT NULL DEFAULT '',
		selected_model TEXT NOT NULL DEFAULT '',
		selected_reasoning_effort TEXT NOT NULL DEFAULT '',
		selected_speed TEXT NOT NULL DEFAULT '',
		selected_acp_options_json TEXT NOT NULL DEFAULT '[]'
			CHECK (json_valid(selected_acp_options_json)),
		runtime_selection_revision INTEGER NOT NULL DEFAULT 0,
		workspace_id   TEXT NOT NULL,
		scope          TEXT NOT NULL DEFAULT 'workspace' CHECK (scope IN ('global', 'workspace')),
		worktree_id    TEXT,
		session_type   TEXT NOT NULL DEFAULT 'user',
		state          TEXT NOT NULL,
		archived_at    TEXT,
		acp_session_id TEXT,
		stop_reason    TEXT,
		stop_escalated BOOLEAN NOT NULL DEFAULT FALSE CHECK (stop_escalated IN (0, 1)),
		stop_verification_failed BOOLEAN NOT NULL DEFAULT FALSE CHECK (stop_verification_failed IN (0, 1)),
		stop_detail    TEXT,
		subprocess_pid INTEGER NOT NULL DEFAULT 0,
		subprocess_started_at TEXT,
		last_update_at TEXT,
		stall_state    TEXT NOT NULL DEFAULT '',
		stall_reason   TEXT NOT NULL DEFAULT '',
		activity_json  TEXT NOT NULL DEFAULT '',
		attached_to    TEXT NOT NULL DEFAULT '',
		attach_expires_at TEXT,
		transcript_epoch INTEGER NOT NULL DEFAULT 0,
		pending_permission_count INTEGER NOT NULL DEFAULT 0 CHECK (pending_permission_count >= 0),
		pending_clarify_count INTEGER NOT NULL DEFAULT 0 CHECK (pending_clarify_count >= 0),
		attention_revision INTEGER NOT NULL DEFAULT 0 CHECK (attention_revision >= 0),
		last_settled_revision INTEGER NOT NULL DEFAULT 0 CHECK (last_settled_revision >= 0),
		last_seen_revision INTEGER NOT NULL DEFAULT 0 CHECK (last_seen_revision >= 0),
		last_seen_at TEXT,
		attention_changed_at TEXT,
		created_at     TEXT NOT NULL,
		updated_at     TEXT NOT NULL
	, failure_kind TEXT, failure_summary TEXT NOT NULL DEFAULT '', crash_bundle_path TEXT NOT NULL DEFAULT '', parent_session_id TEXT, root_session_id TEXT, spawn_depth INTEGER NOT NULL DEFAULT 0, spawn_role TEXT, ttl_expires_at TEXT, auto_stop_on_parent BOOLEAN NOT NULL DEFAULT 0, notify_creator BOOLEAN NOT NULL DEFAULT 1, spawn_budget_json TEXT NOT NULL DEFAULT '{}', permission_policy_json TEXT NOT NULL DEFAULT '{}', soul_snapshot_id TEXT
				REFERENCES agent_soul_snapshots(id) ON DELETE SET NULL, soul_digest TEXT NOT NULL DEFAULT '', parent_soul_digest TEXT NOT NULL DEFAULT '', input_generation INTEGER NOT NULL DEFAULT 0, creation_digest TEXT
				CHECK (creation_digest IS NULL OR length(trim(creation_digest)) > 0), policy_spec_digest TEXT
				CHECK (policy_spec_digest IS NULL OR length(trim(policy_spec_digest)) > 0), creation_profile_ref TEXT
				CHECK (creation_profile_ref IS NULL OR length(trim(creation_profile_ref)) > 0),
		FOREIGN KEY (workspace_id, worktree_id)
			REFERENCES worktrees(workspace_id, id),
		CHECK ((scope = 'workspace') = (workspace_id <> '')),
		UNIQUE (workspace_id, id));

INSERT INTO "new_sessions" ("id", "profile_id", "name", "agent_name", "provider", "model", "reasoning_effort", "speed", "acp_options_json", "speed_resolution_json", "runtime_status", "runtime_transition", "runtime_failure", "runtime_generation", "runtime_recovery_json", "selected_provider", "selected_model", "selected_reasoning_effort", "selected_speed", "selected_acp_options_json", "runtime_selection_revision", "workspace_id", "scope", "worktree_id", "session_type", "state", "archived_at", "acp_session_id", "stop_reason", "stop_escalated", "stop_verification_failed", "stop_detail", "subprocess_pid", "subprocess_started_at", "last_update_at", "stall_state", "stall_reason", "activity_json", "attached_to", "attach_expires_at", "transcript_epoch", "pending_permission_count", "pending_clarify_count", "attention_revision", "last_settled_revision", "last_seen_revision", "last_seen_at", "attention_changed_at", "created_at", "updated_at", "failure_kind", "failure_summary", "crash_bundle_path", "parent_session_id", "root_session_id", "spawn_depth", "spawn_role", "ttl_expires_at", "auto_stop_on_parent", "notify_creator", "spawn_budget_json", "permission_policy_json", "soul_snapshot_id", "soul_digest", "parent_soul_digest", "input_generation", "creation_digest", "policy_spec_digest", "creation_profile_ref") SELECT "id", "profile_id", "name", "agent_name", "provider", "model", "reasoning_effort", "speed", "acp_options_json", "speed_resolution_json", "runtime_status", "runtime_transition", "runtime_failure", "runtime_generation", "runtime_recovery_json", "selected_provider", "selected_model", "selected_reasoning_effort", "selected_speed", "selected_acp_options_json", "runtime_selection_revision", "workspace_id", "scope", "worktree_id", "session_type", "state", "archived_at", "acp_session_id", "stop_reason", "stop_escalated", "stop_verification_failed", "stop_detail", "subprocess_pid", "subprocess_started_at", "last_update_at", "stall_state", "stall_reason", "activity_json", "attached_to", "attach_expires_at", "transcript_epoch", "pending_permission_count", "pending_clarify_count", "attention_revision", "last_settled_revision", "last_seen_revision", "last_seen_at", "attention_changed_at", "created_at", "updated_at", "failure_kind", "failure_summary", "crash_bundle_path", "parent_session_id", "root_session_id", "spawn_depth", "spawn_role", "ttl_expires_at", "auto_stop_on_parent", "notify_creator", "spawn_budget_json", "permission_policy_json", "soul_snapshot_id", "soul_digest", "parent_soul_digest", "input_generation", "creation_digest", "policy_spec_digest", "creation_profile_ref" FROM "sessions";

DROP TABLE "sessions";

ALTER TABLE "new_sessions" RENAME TO "sessions";

CREATE INDEX idx_sessions_attach_lock
			ON sessions(attached_to, attach_expires_at);

CREATE INDEX idx_sessions_catalog_activity
			ON sessions(
				workspace_id, state, COALESCE(last_update_at, updated_at) DESC,
				updated_at DESC, created_at DESC, id DESC
			);

CREATE INDEX idx_sessions_catalog_archive_recent
			ON sessions(
				workspace_id, archived_at, state, updated_at DESC, created_at DESC, id DESC
			);

CREATE INDEX idx_sessions_catalog_recent
			ON sessions(workspace_id, state, updated_at DESC, created_at DESC, id DESC);

CREATE INDEX idx_sessions_parent ON sessions(parent_session_id);

CREATE INDEX idx_sessions_profile_catalog_activity
			ON sessions(
				profile_id, workspace_id, state, COALESCE(last_update_at, updated_at) DESC,
				updated_at DESC, created_at DESC, id DESC
			);

CREATE INDEX idx_sessions_profile_catalog_archive_recent
			ON sessions(
				profile_id, workspace_id, archived_at, state, updated_at DESC, created_at DESC, id DESC
			);

CREATE INDEX idx_sessions_profile_catalog_recent
			ON sessions(profile_id, workspace_id, state, updated_at DESC, created_at DESC, id DESC);

CREATE INDEX idx_sessions_resumable
			ON sessions(state, failure_kind, last_update_at, updated_at);

CREATE INDEX idx_sessions_root ON sessions(root_session_id);

CREATE INDEX idx_sessions_soul_snapshot
			ON sessions(soul_snapshot_id);

CREATE INDEX idx_sessions_spawn_role ON sessions(spawn_role);

CREATE INDEX idx_sessions_type_depth ON sessions(session_type, spawn_depth);

CREATE INDEX idx_sessions_worktree ON sessions(worktree_id) WHERE worktree_id IS NOT NULL;

CREATE TABLE "new_task_events" (
		id          TEXT PRIMARY KEY,
		event_seq   INTEGER NOT NULL,
		task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		run_id      TEXT REFERENCES task_runs(id) ON DELETE SET NULL,
		event_type  TEXT NOT NULL,
		actor_kind  TEXT NOT NULL CHECK (
			actor_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'daemon'
			)
		),
		actor_id    TEXT NOT NULL,
		origin_kind TEXT NOT NULL CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'agent_session', 'daemon'
			)
		),
		origin_ref  TEXT NOT NULL,
		payload_json TEXT,
		timestamp   TEXT NOT NULL
	);

INSERT INTO "new_task_events" ("id", "event_seq", "task_id", "run_id", "event_type", "actor_kind", "actor_id", "origin_kind", "origin_ref", "payload_json", "timestamp") SELECT "id", "event_seq", "task_id", "run_id", "event_type", "actor_kind", "actor_id", "origin_kind", "origin_ref", "payload_json", "timestamp" FROM "task_events";

DROP TABLE "task_events";

ALTER TABLE "new_task_events" RENAME TO "task_events";

CREATE INDEX idx_task_events_run ON task_events(run_id, timestamp DESC, id DESC);

CREATE INDEX idx_task_events_task ON task_events(task_id, timestamp DESC, id DESC);

CREATE INDEX idx_task_events_task_seq ON task_events(task_id, event_seq ASC);

CREATE INDEX idx_task_events_type ON task_events(event_type, timestamp DESC, id DESC);

CREATE INDEX idx_task_events_type_seq
ON task_events(event_type, event_seq);

CREATE INDEX idx_task_events_wake_event
ON task_events(task_id, event_type, json_extract(payload_json, '$.wake_event_id'))
WHERE event_type IN ('task.wake.delivered', 'task.wake.suppressed');

CREATE UNIQUE INDEX uq_task_events_event_seq ON task_events(event_seq);

CREATE TABLE "new_task_execution_profiles" (
			task_id                  TEXT PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
			coordinator_mode         TEXT NOT NULL DEFAULT 'inherit' CHECK (
				coordinator_mode IN ('inherit', 'guided')
			),
			coordinator_agent_name   TEXT NOT NULL DEFAULT '',
			coordinator_provider     TEXT NOT NULL DEFAULT '',
			coordinator_model        TEXT NOT NULL DEFAULT '',
			coordinator_guidance     TEXT NOT NULL DEFAULT '',
			worker_mode              TEXT NOT NULL DEFAULT 'inherit' CHECK (
				worker_mode IN ('inherit', 'select')
			),
			worker_agent_name        TEXT NOT NULL DEFAULT '',
			worker_provider          TEXT NOT NULL DEFAULT '',
			worker_model             TEXT NOT NULL DEFAULT '',
			worker_reasoning_effort  TEXT NOT NULL DEFAULT '',
			worker_speed             TEXT NOT NULL DEFAULT '' CHECK (worker_speed IN ('', 'normal', 'fast')),
			worker_acp_options_json  TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(worker_acp_options_json)),
			review_agent_name        TEXT NOT NULL DEFAULT '',
			review_provider          TEXT NOT NULL DEFAULT '',
			review_model             TEXT NOT NULL DEFAULT '',
			review_reasoning_effort  TEXT NOT NULL DEFAULT '',
			review_speed             TEXT NOT NULL DEFAULT '' CHECK (review_speed IN ('', 'normal', 'fast')),
			review_acp_options_json  TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(review_acp_options_json)),
			worktree_mode            TEXT NOT NULL DEFAULT 'inherit' CHECK (
				worktree_mode IN ('inherit', 'none', 'ref', 'per_run')
			),
			worktree_ref             TEXT NOT NULL DEFAULT '',
			created_at               TEXT NOT NULL,
			updated_at               TEXT NOT NULL,
			runtime_mode             TEXT NOT NULL DEFAULT 'default' CHECK (
				runtime_mode IN ('default', 'evidence')
			),
			CHECK (
				(worktree_mode = 'ref') = (worktree_ref <> '')
			)
		);

INSERT INTO "new_task_execution_profiles" ("task_id", "coordinator_mode", "coordinator_agent_name", "coordinator_provider", "coordinator_model", "coordinator_guidance", "worker_mode", "worker_agent_name", "worker_provider", "worker_model", "worker_reasoning_effort", "worker_speed", "worker_acp_options_json", "review_agent_name", "review_provider", "review_model", "review_reasoning_effort", "review_speed", "review_acp_options_json", "worktree_mode", "worktree_ref", "created_at", "updated_at", "runtime_mode") SELECT "task_id", "coordinator_mode", "coordinator_agent_name", "coordinator_provider", "coordinator_model", "coordinator_guidance", "worker_mode", "worker_agent_name", "worker_provider", "worker_model", "worker_reasoning_effort", "worker_speed", "worker_acp_options_json", "review_agent_name", "review_provider", "review_model", "review_reasoning_effort", "review_speed", "review_acp_options_json", "worktree_mode", "worktree_ref", "created_at", "updated_at", "runtime_mode" FROM "task_execution_profiles";

DROP TABLE "task_execution_profiles";

ALTER TABLE "new_task_execution_profiles" RENAME TO "task_execution_profiles";

CREATE INDEX task_execution_profiles_task_id_idx
			ON task_execution_profiles(task_id);

CREATE TABLE "new_task_run_idempotency" (
		idempotency_key TEXT NOT NULL,
		origin_kind     TEXT NOT NULL CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'agent_session', 'daemon'
			)
		),
		origin_ref      TEXT NOT NULL,
		run_id          TEXT NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
		created_at      TEXT NOT NULL,
		PRIMARY KEY (idempotency_key, origin_kind, origin_ref)
	);

INSERT INTO "new_task_run_idempotency" ("idempotency_key", "origin_kind", "origin_ref", "run_id", "created_at") SELECT "idempotency_key", "origin_kind", "origin_ref", "run_id", "created_at" FROM "task_run_idempotency";

DROP TABLE "task_run_idempotency";

ALTER TABLE "new_task_run_idempotency" RENAME TO "task_run_idempotency";

CREATE INDEX idx_task_run_idempotency_run ON task_run_idempotency(run_id);

CREATE TABLE "new_task_run_reviews" (
		review_id            TEXT PRIMARY KEY,
		task_id              TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		run_id               TEXT NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
		parent_review_id     TEXT REFERENCES task_run_reviews(review_id) ON DELETE SET NULL,
		policy               TEXT NOT NULL CHECK (policy IN ('none', 'on_success', 'on_failure', 'always')),
		review_round         INTEGER NOT NULL CHECK (review_round >= 0),
		attempt              INTEGER NOT NULL CHECK (attempt > 0),
		status               TEXT NOT NULL CHECK (
			status IN ('requested', 'routed', 'in_review', 'recorded', 'circuit_opened', 'canceled')
		),
		outcome              TEXT CHECK (
			outcome IS NULL OR outcome IN (
				'approved', 'rejected', 'blocked', 'error', 'timeout', 'invalid_output'
			)
		),
		confidence           REAL CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
		reason               TEXT NOT NULL DEFAULT '',
		delivery_id          TEXT,
		missing_work_json    TEXT NOT NULL DEFAULT '[]',
		next_round_guidance  TEXT NOT NULL DEFAULT '',
		review_text          TEXT NOT NULL DEFAULT '',
		reviewer_session_id  TEXT,
		reviewer_agent_name  TEXT NOT NULL DEFAULT '',
		reviewed_by_kind     TEXT NOT NULL DEFAULT '',
		reviewed_by_ref      TEXT NOT NULL DEFAULT '',
		requested_at         TEXT NOT NULL,
		routed_at            TEXT,
		started_at           TEXT,
		reviewed_at          TEXT,
		deadline_at          TEXT,
		created_at           TEXT NOT NULL,
		updated_at           TEXT NOT NULL
	);

INSERT INTO "new_task_run_reviews" ("review_id", "task_id", "run_id", "parent_review_id", "policy", "review_round", "attempt", "status", "outcome", "confidence", "reason", "delivery_id", "missing_work_json", "next_round_guidance", "review_text", "reviewer_session_id", "reviewer_agent_name", "reviewed_by_kind", "reviewed_by_ref", "requested_at", "routed_at", "started_at", "reviewed_at", "deadline_at", "created_at", "updated_at") SELECT "review_id", "task_id", "run_id", "parent_review_id", "policy", "review_round", "attempt", "status", "outcome", "confidence", "reason", "delivery_id", "missing_work_json", "next_round_guidance", "review_text", "reviewer_session_id", "reviewer_agent_name", "reviewed_by_kind", "reviewed_by_ref", "requested_at", "routed_at", "started_at", "reviewed_at", "deadline_at", "created_at", "updated_at" FROM "task_run_reviews";

DROP TABLE "task_run_reviews";

ALTER TABLE "new_task_run_reviews" RENAME TO "task_run_reviews";

CREATE INDEX idx_task_run_reviews_deadline
		ON task_run_reviews(status, deadline_at);

CREATE INDEX idx_task_run_reviews_reviewer_agent
		ON task_run_reviews(reviewer_agent_name, status);

CREATE INDEX idx_task_run_reviews_reviewer_session
		ON task_run_reviews(reviewer_session_id, status);

CREATE INDEX idx_task_run_reviews_run_status
		ON task_run_reviews(run_id, status);

CREATE INDEX idx_task_run_reviews_task_round_attempt
		ON task_run_reviews(task_id, review_round, attempt);

CREATE UNIQUE INDEX uq_task_run_reviews_delivery
		ON task_run_reviews(review_id, delivery_id)
		WHERE delivery_id IS NOT NULL;

CREATE UNIQUE INDEX uq_task_run_reviews_reviewer_session_active
		ON task_run_reviews(reviewer_session_id)
		WHERE reviewer_session_id IS NOT NULL AND status IN ('routed', 'in_review');

CREATE UNIQUE INDEX uq_task_run_reviews_run_round_attempt
		ON task_run_reviews(run_id, review_round, attempt);

CREATE TABLE "new_task_runs" (
		id              TEXT PRIMARY KEY,
		task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		workspace_id    TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		worktree_id     TEXT,
		status          TEXT NOT NULL,
		attempt         INTEGER NOT NULL CHECK (attempt > 0),
		recovery_count  INTEGER NOT NULL DEFAULT 0 CHECK (recovery_count >= 0),
		previous_run_id TEXT,
		failure_kind    TEXT NOT NULL DEFAULT '' CHECK (
			failure_kind = '' OR failure_kind IN ('operator_forced')
		),
		claimed_by_kind TEXT CHECK (
			claimed_by_kind IS NULL OR claimed_by_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'daemon'
			)
		),
		claimed_by_ref  TEXT,
		session_id      TEXT,
		origin_kind     TEXT NOT NULL CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'agent_session', 'daemon'
			)
		),
		origin_ref      TEXT NOT NULL,
		idempotency_key TEXT,
		designation_group_id TEXT NOT NULL DEFAULT '',
		resolved_worktree_mode TEXT NOT NULL DEFAULT '' CHECK (
			resolved_worktree_mode IN ('', 'none', 'ref', 'per_run')
		),
		resolved_worktree_ref TEXT NOT NULL DEFAULT '',
		queued_at       TEXT NOT NULL,
		claimed_at      TEXT,
		started_at      TEXT,
		ended_at        TEXT,
		error           TEXT,
		metadata_json   TEXT,
		result_json     TEXT,
		summary         TEXT NOT NULL DEFAULT '',
		claimed_agent_name TEXT NOT NULL DEFAULT '',
		terminalized_by_session_id TEXT NOT NULL DEFAULT '',
		terminalized_by_agent_name TEXT NOT NULL DEFAULT '',
		terminalized_by_actor_kind TEXT NOT NULL DEFAULT '',
		terminalized_by_actor_ref TEXT NOT NULL DEFAULT '',
		review_required BOOLEAN NOT NULL DEFAULT 0 CHECK (review_required IN (0, 1)),
		review_request_round INTEGER NOT NULL DEFAULT 0 CHECK (review_request_round >= 0),
		review_policy_snapshot TEXT NOT NULL DEFAULT '' CHECK (
			review_policy_snapshot = '' OR
			review_policy_snapshot IN ('none', 'on_success', 'on_failure', 'always')
		),
		review_request_id TEXT REFERENCES task_run_reviews(review_id),
		parent_run_id TEXT REFERENCES task_runs(id),
		review_id TEXT REFERENCES task_run_reviews(review_id),
		review_round INTEGER NOT NULL DEFAULT 0 CHECK (review_round >= 0),
		continuation_reason TEXT NOT NULL DEFAULT '',
		missing_work_json TEXT NOT NULL DEFAULT '[]',
		next_round_guidance TEXT NOT NULL DEFAULT '',
		claim_token TEXT,
		claim_token_hash TEXT,
		lease_until TEXT,
		heartbeat_at TEXT,
		run_kind TEXT NOT NULL DEFAULT 'worker' CHECK (run_kind IN ('worker', 'coordinator')),
		loop_run_id TEXT,
		tokens_used INTEGER NOT NULL DEFAULT 0 CHECK (tokens_used >= 0),
		CHECK (
			(claimed_by_kind IS NULL AND claimed_by_ref IS NULL) OR
			(claimed_by_kind IS NOT NULL AND claimed_by_ref IS NOT NULL)
		),
		CHECK (status <> 'queued' OR session_id IS NULL),
		CHECK (
			(resolved_worktree_mode = 'ref') = (resolved_worktree_ref <> '')
		),
		FOREIGN KEY (workspace_id, worktree_id)
			REFERENCES worktrees(workspace_id, id),
		UNIQUE (workspace_id, id)
	);

INSERT INTO "new_task_runs" ("id", "task_id", "workspace_id", "worktree_id", "status", "attempt", "recovery_count", "previous_run_id", "failure_kind", "claimed_by_kind", "claimed_by_ref", "session_id", "origin_kind", "origin_ref", "idempotency_key", "designation_group_id", "resolved_worktree_mode", "resolved_worktree_ref", "queued_at", "claimed_at", "started_at", "ended_at", "error", "metadata_json", "result_json", "summary", "claimed_agent_name", "terminalized_by_session_id", "terminalized_by_agent_name", "terminalized_by_actor_kind", "terminalized_by_actor_ref", "review_required", "review_request_round", "review_policy_snapshot", "review_request_id", "parent_run_id", "review_id", "review_round", "continuation_reason", "missing_work_json", "next_round_guidance", "claim_token", "claim_token_hash", "lease_until", "heartbeat_at", "run_kind", "loop_run_id", "tokens_used") SELECT "id", "task_id", "workspace_id", "worktree_id", "status", "attempt", "recovery_count", "previous_run_id", "failure_kind", "claimed_by_kind", "claimed_by_ref", "session_id", "origin_kind", "origin_ref", "idempotency_key", "designation_group_id", "resolved_worktree_mode", "resolved_worktree_ref", "queued_at", "claimed_at", "started_at", "ended_at", "error", "metadata_json", "result_json", "summary", "claimed_agent_name", "terminalized_by_session_id", "terminalized_by_agent_name", "terminalized_by_actor_kind", "terminalized_by_actor_ref", "review_required", "review_request_round", "review_policy_snapshot", "review_request_id", "parent_run_id", "review_id", "review_round", "continuation_reason", "missing_work_json", "next_round_guidance", "claim_token", "claim_token_hash", "lease_until", "heartbeat_at", "run_kind", "loop_run_id", "tokens_used" FROM "task_runs";

DROP TABLE "task_runs";

ALTER TABLE "new_task_runs" RENAME TO "task_runs";

CREATE INDEX idx_task_runs_active_lease_recovery
			ON task_runs(status, lease_until, heartbeat_at, id);

CREATE INDEX idx_task_runs_designation_group ON task_runs(task_id, designation_group_id);

CREATE INDEX idx_task_runs_parent_run ON task_runs(parent_run_id);

CREATE INDEX idx_task_runs_pending_claim
			ON task_runs(status, lease_until, queued_at, id);

CREATE INDEX idx_task_runs_previous ON task_runs(previous_run_id);

CREATE INDEX idx_task_runs_review_request
		ON task_runs(review_request_id)
		WHERE review_request_id IS NOT NULL;

CREATE INDEX idx_task_runs_session ON task_runs(session_id);

CREATE INDEX idx_task_runs_session_status
			ON task_runs(session_id, status, lease_until);

CREATE INDEX idx_task_runs_status ON task_runs(status);

CREATE INDEX idx_task_runs_task ON task_runs(task_id, queued_at DESC, id DESC);

CREATE INDEX idx_task_runs_task_review_round
		ON task_runs(task_id, review_round)
		WHERE review_round > 0;

CREATE INDEX idx_task_runs_task_status ON task_runs(task_id, status, queued_at DESC, id DESC);

CREATE INDEX idx_task_runs_workspace_active
			ON task_runs(workspace_id, status, lease_until)
			WHERE workspace_id IS NOT NULL AND run_kind IN ('worker', 'coordinator');

CREATE INDEX idx_task_runs_worktree ON task_runs(worktree_id) WHERE worktree_id IS NOT NULL;

CREATE UNIQUE INDEX uq_task_runs_active_loop_coordinator
			ON task_runs(loop_run_id)
			WHERE run_kind = 'coordinator' AND status IN ('queued', 'claimed', 'starting', 'running');

CREATE UNIQUE INDEX uq_task_runs_review_id
		ON task_runs(review_id)
		WHERE review_id IS NOT NULL;

CREATE TABLE "new_task_triage_state" (
		task_id               TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		actor_kind            TEXT NOT NULL CHECK (
			actor_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'daemon'
			)
		),
		actor_id              TEXT NOT NULL,
		is_read               BOOLEAN NOT NULL DEFAULT 0,
		archived              BOOLEAN NOT NULL DEFAULT 0,
		dismissed             BOOLEAN NOT NULL DEFAULT 0,
		last_seen_activity_at TEXT,
		updated_at            TEXT NOT NULL,
		PRIMARY KEY (task_id, actor_kind, actor_id)
	);

INSERT INTO "new_task_triage_state" ("task_id", "actor_kind", "actor_id", "is_read", "archived", "dismissed", "last_seen_activity_at", "updated_at") SELECT "task_id", "actor_kind", "actor_id", "is_read", "archived", "dismissed", "last_seen_activity_at", "updated_at" FROM "task_triage_state";

DROP TABLE "task_triage_state";

ALTER TABLE "new_task_triage_state" RENAME TO "task_triage_state";

CREATE INDEX idx_task_triage_actor ON task_triage_state(actor_kind, actor_id, updated_at DESC, task_id);

CREATE INDEX idx_task_triage_task ON task_triage_state(task_id, updated_at DESC);

CREATE TABLE "new_tasks" (
		id              TEXT PRIMARY KEY,
		profile_id      TEXT NOT NULL REFERENCES profiles(id),
		identifier      TEXT,
		scope           TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
		workspace_id    TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		parent_task_id  TEXT REFERENCES tasks(id),
		title           TEXT NOT NULL,
		description     TEXT,
		priority        TEXT NOT NULL DEFAULT 'medium' CHECK (
			priority IN ('low', 'medium', 'high', 'urgent')
		),
		max_attempts    INTEGER NOT NULL DEFAULT 3 CHECK (max_attempts > 0 AND max_attempts <= 10),
		status          TEXT NOT NULL,
		approval_policy TEXT NOT NULL DEFAULT 'none' CHECK (
			approval_policy IN ('none', 'manual')
		),
		approval_state  TEXT NOT NULL DEFAULT 'not_required' CHECK (
			approval_state IN ('not_required', 'pending', 'approved', 'rejected')
		),
		owner_kind      TEXT CHECK (
			owner_kind IS NULL OR owner_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'pool'
			)
		),
		owner_ref       TEXT,
		created_by_kind TEXT NOT NULL CHECK (
			created_by_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'daemon'
			)
		),
		created_by_ref  TEXT NOT NULL,
		origin_kind     TEXT NOT NULL CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'agent_session', 'daemon'
			)
		),
		origin_ref      TEXT NOT NULL,
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL,
		closed_at       TEXT,
		metadata_json   TEXT,
		current_run_id  TEXT REFERENCES task_runs(id) ON DELETE SET NULL,
		paused          INTEGER NOT NULL DEFAULT 0 CHECK (paused IN (0, 1)),
		paused_by       TEXT NOT NULL DEFAULT '',
		paused_at       TEXT,
		paused_reason   TEXT NOT NULL DEFAULT '',
		max_runtime_seconds INTEGER NOT NULL DEFAULT 0 CHECK (max_runtime_seconds >= 0),
		spawn_failure_count INTEGER NOT NULL DEFAULT 0 CHECK (spawn_failure_count >= 0),
		last_spawn_error TEXT NOT NULL DEFAULT '',
		review_policy TEXT NOT NULL DEFAULT 'none' CHECK (
			review_policy IN ('none', 'on_success', 'on_failure', 'always')
		),
		review_max_rounds INTEGER NOT NULL DEFAULT 3 CHECK (review_max_rounds >= 0),
		review_round INTEGER NOT NULL DEFAULT 0 CHECK (review_round >= 0),
		last_review_id TEXT,
		last_review_outcome TEXT CHECK (
			last_review_outcome IS NULL OR last_review_outcome IN (
				'approved', 'rejected', 'blocked', 'error', 'timeout', 'invalid_output'
			)
		),
		review_circuit_opened_at TEXT,
		review_circuit_reason TEXT,
		auto_enqueue_on_ready INTEGER NOT NULL DEFAULT 0 CHECK (auto_enqueue_on_ready IN (0, 1)),
		needs_attention_reason  TEXT,
		needs_attention_at      TEXT,
		needs_attention_by_kind TEXT,
		needs_attention_by_ref  TEXT,
		wake_creator            INTEGER NOT NULL DEFAULT 1,
		CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		),
		CHECK (
			(owner_kind IS NULL AND owner_ref IS NULL) OR
			(owner_kind IS NOT NULL AND owner_ref IS NOT NULL)
		),
		CHECK (parent_task_id IS NULL OR parent_task_id <> id),
		CHECK (
			(approval_policy = 'none' AND approval_state = 'not_required') OR
			(approval_policy = 'manual' AND approval_state IN ('pending', 'approved', 'rejected'))
		)
		);

INSERT INTO "new_tasks" ("id", "profile_id", "identifier", "scope", "workspace_id", "parent_task_id", "title", "description", "priority", "max_attempts", "status", "approval_policy", "approval_state", "owner_kind", "owner_ref", "created_by_kind", "created_by_ref", "origin_kind", "origin_ref", "created_at", "updated_at", "closed_at", "metadata_json", "current_run_id", "paused", "paused_by", "paused_at", "paused_reason", "max_runtime_seconds", "spawn_failure_count", "last_spawn_error", "review_policy", "review_max_rounds", "review_round", "last_review_id", "last_review_outcome", "review_circuit_opened_at", "review_circuit_reason", "auto_enqueue_on_ready", "needs_attention_reason", "needs_attention_at", "needs_attention_by_kind", "needs_attention_by_ref", "wake_creator") SELECT "id", "profile_id", "identifier", "scope", "workspace_id", "parent_task_id", "title", "description", "priority", "max_attempts", "status", "approval_policy", "approval_state", "owner_kind", "owner_ref", "created_by_kind", "created_by_ref", "origin_kind", "origin_ref", "created_at", "updated_at", "closed_at", "metadata_json", "current_run_id", "paused", "paused_by", "paused_at", "paused_reason", "max_runtime_seconds", "spawn_failure_count", "last_spawn_error", "review_policy", "review_max_rounds", "review_round", "last_review_id", "last_review_outcome", "review_circuit_opened_at", "review_circuit_reason", "auto_enqueue_on_ready", "needs_attention_reason", "needs_attention_at", "needs_attention_by_kind", "needs_attention_by_ref", "wake_creator" FROM "tasks";

DROP TABLE "tasks";

ALTER TABLE "new_tasks" RENAME TO "tasks";

CREATE INDEX idx_tasks_approval_state ON tasks(approval_state);

CREATE INDEX idx_tasks_created_by ON tasks(created_by_kind, created_by_ref);

CREATE INDEX idx_tasks_current_run ON tasks(current_run_id);

CREATE INDEX idx_tasks_owner ON tasks(owner_kind, owner_ref);

CREATE INDEX idx_tasks_parent ON tasks(parent_task_id);

CREATE INDEX idx_tasks_paused ON tasks(paused, updated_at DESC);

CREATE INDEX idx_tasks_priority ON tasks(priority);

CREATE INDEX idx_tasks_review_policy ON tasks(review_policy);

CREATE INDEX idx_tasks_review_round ON tasks(review_round);

CREATE INDEX idx_tasks_scope ON tasks(scope);

CREATE INDEX idx_tasks_status ON tasks(status);

CREATE INDEX idx_tasks_workspace ON tasks(workspace_id);

CREATE TABLE "new_tool_processes" (
			id               TEXT PRIMARY KEY,
			source           TEXT NOT NULL,
			session_id       TEXT NOT NULL DEFAULT '',
			turn_id          TEXT NOT NULL DEFAULT '',
			tool_call_id     TEXT NOT NULL DEFAULT '',
			terminal_id      TEXT NOT NULL DEFAULT '',
			extension_name   TEXT NOT NULL DEFAULT '',
			hook_name        TEXT NOT NULL DEFAULT '',
			pid              INTEGER NOT NULL DEFAULT 0,
			process_group_id INTEGER NOT NULL DEFAULT 0,
			command          TEXT NOT NULL DEFAULT '',
			args_json        TEXT NOT NULL DEFAULT '[]',
			cwd              TEXT NOT NULL DEFAULT '',
			started_at       TEXT,
			started_by_pid   INTEGER NOT NULL DEFAULT 0,
			state            TEXT NOT NULL,
			exit_code        INTEGER,
			error            TEXT NOT NULL DEFAULT '',
			created_at       TEXT NOT NULL,
			updated_at       TEXT NOT NULL,
			completed_at     TEXT
		);

INSERT INTO "new_tool_processes" ("id", "source", "session_id", "turn_id", "tool_call_id", "terminal_id", "extension_name", "hook_name", "pid", "process_group_id", "command", "args_json", "cwd", "started_at", "started_by_pid", "state", "exit_code", "error", "created_at", "updated_at", "completed_at") SELECT "id", "source", "session_id", "turn_id", "tool_call_id", "terminal_id", "extension_name", "hook_name", "pid", "process_group_id", "command", "args_json", "cwd", "started_at", "started_by_pid", "state", "exit_code", "error", "created_at", "updated_at", "completed_at" FROM "tool_processes";

DROP TABLE "tool_processes";

ALTER TABLE "new_tool_processes" RENAME TO "tool_processes";

CREATE INDEX idx_tool_processes_extension
			ON tool_processes(extension_name);

CREATE INDEX idx_tool_processes_hook
			ON tool_processes(hook_name);

CREATE INDEX idx_tool_processes_session_turn
			ON tool_processes(session_id, turn_id);

CREATE INDEX idx_tool_processes_state_updated
			ON tool_processes(state, updated_at);

CREATE INDEX idx_tool_processes_terminal
			ON tool_processes(terminal_id);

CREATE INDEX idx_tool_processes_tool_call
			ON tool_processes(tool_call_id);

CREATE TABLE "new_workspace_deletion_intents" (
    workspace_id  TEXT NOT NULL PRIMARY KEY,
    root_dir      TEXT NOT NULL,
    add_dirs      TEXT NOT NULL,
    name          TEXT NOT NULL,
    default_agent TEXT,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    requested_at  TEXT NOT NULL
);

INSERT INTO "new_workspace_deletion_intents" ("workspace_id", "root_dir", "add_dirs", "name", "default_agent", "created_at", "updated_at", "requested_at") SELECT "workspace_id", "root_dir", "add_dirs", "name", "default_agent", "created_at", "updated_at", "requested_at" FROM "workspace_deletion_intents";

DROP TABLE "workspace_deletion_intents";

ALTER TABLE "new_workspace_deletion_intents" RENAME TO "workspace_deletion_intents";

CREATE INDEX idx_workspace_deletion_intents_requested
ON workspace_deletion_intents(requested_at, workspace_id);

CREATE TABLE "new_workspaces" (
		id            TEXT PRIMARY KEY,
		root_dir      TEXT NOT NULL UNIQUE,
		add_dirs      TEXT NOT NULL DEFAULT '[]',
		name          TEXT NOT NULL UNIQUE,
		default_agent TEXT DEFAULT '',
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL
	);

INSERT INTO "new_workspaces" ("id", "root_dir", "add_dirs", "name", "default_agent", "created_at", "updated_at") SELECT "id", "root_dir", "add_dirs", "name", "default_agent", "created_at", "updated_at" FROM "workspaces";

DROP TABLE "workspaces";

ALTER TABLE "new_workspaces" RENAME TO "workspaces";

CREATE INDEX idx_workspaces_name ON workspaces(name);

DROP TABLE "bridge_deliveries";

DROP TABLE "bridge_delivery_metrics";

DROP TABLE "bridge_ingest_dedup";

DROP TABLE "bridge_instances";

DROP TABLE "bridge_routes";

DROP TABLE "bridge_secret_bindings";

DROP TABLE "bridge_target_directory";

DROP TABLE "bridge_target_directory_refresh";

DROP TABLE "bridge_task_subscriptions";

DROP TABLE "network_audit_log";

DROP TABLE "network_availability";

DROP TABLE "network_channel_kind_counts";

DROP TABLE "network_channel_participants";

DROP TABLE "network_channel_stats";

DROP TABLE "network_channels";

DROP TABLE "network_coordination_invitations";

DROP TABLE "network_direct_rooms";

DROP TABLE "network_live_wakes";

DROP TABLE "network_message_dispositions";

DROP TABLE "network_participation_budgets";

DROP TABLE "network_subscriptions";

DROP TABLE "network_task_status_projections";

DROP TABLE "network_task_thread_origins";

DROP TABLE "network_thread_participants";

DROP TABLE "network_thread_session_token_stats";

DROP TABLE "network_threads";

DROP TABLE "network_timeline_log";

DROP TABLE "network_wake_events";

DROP TABLE "network_wake_sources";

DROP TABLE "network_work";

DROP TABLE "notification_delivery_permits";

DROP TABLE "notification_preset_enablement";

DROP TABLE "notification_presets";

DROP TABLE "task_network_coordination";

DROP TABLE "task_profile_channels";

DROP TABLE "task_profile_peers";

DROP TABLE "workspace_network_coordination";

-- +goose StatementBegin

CREATE TRIGGER agent_heartbeat_revisions_workspace_insert_guard
BEFORE INSERT ON agent_heartbeat_revisions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER agent_heartbeat_revisions_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_heartbeat_revisions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER agent_heartbeat_snapshots_workspace_insert_guard
BEFORE INSERT ON agent_heartbeat_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER agent_heartbeat_snapshots_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_heartbeat_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER agent_heartbeat_wake_events_workspace_insert_guard
BEFORE INSERT ON agent_heartbeat_wake_events
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER agent_heartbeat_wake_events_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_heartbeat_wake_events
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER agent_heartbeat_wake_state_workspace_insert_guard
BEFORE INSERT ON agent_heartbeat_wake_state
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER agent_heartbeat_wake_state_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_heartbeat_wake_state
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER agent_soul_revisions_workspace_insert_guard
BEFORE INSERT ON agent_soul_revisions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER agent_soul_revisions_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_soul_revisions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER agent_soul_snapshots_workspace_insert_guard
BEFORE INSERT ON agent_soul_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER agent_soul_snapshots_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_soul_snapshots
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER automation_jobs_profile_owner_active BEFORE INSERT ON automation_jobs BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER automation_jobs_profile_owner_immutable BEFORE UPDATE OF profile_id ON automation_jobs
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER automation_triggers_profile_owner_active BEFORE INSERT ON automation_triggers BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER automation_triggers_profile_owner_immutable BEFORE UPDATE OF profile_id ON automation_triggers
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;

-- +goose StatementEnd

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

-- +goose StatementBegin

CREATE TRIGGER cmd_palette_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM cmd_palette_usage WHERE workspace_id = OLD.id;
	DELETE FROM cmd_palette_query_hits WHERE workspace_id = OLD.id;
	DELETE FROM cmd_palette_pins WHERE workspace_id = OLD.id;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER dead_entities_profile_owner_active BEFORE INSERT ON dead_entities BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER dead_entities_profile_owner_immutable BEFORE UPDATE OF profile_id ON dead_entities
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER dead_entities_workspace_insert_guard
BEFORE INSERT ON dead_entities
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER dead_entities_workspace_update_guard
BEFORE UPDATE OF workspace_id ON dead_entities
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER extension_dev_links_profile_enablement_delete
AFTER DELETE ON extension_dev_links
WHEN NOT EXISTS (
		SELECT 1 FROM extensions WHERE name = OLD.extension_name
	)
	AND NOT EXISTS (
		SELECT 1 FROM extension_dev_links WHERE extension_name = OLD.extension_name
	)
BEGIN
	DELETE FROM extension_profile_enablement WHERE extension_name = OLD.extension_name;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER extension_dev_links_profile_enablement_insert
BEFORE INSERT ON extension_profile_enablement
WHEN NOT EXISTS (
		SELECT 1 FROM extensions WHERE name = NEW.extension_name
	)
	AND NOT EXISTS (
		SELECT 1 FROM extension_dev_links WHERE extension_name = NEW.extension_name
	)
BEGIN
	SELECT RAISE(ABORT, 'extension_not_found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER extension_env_bindings_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM extension_env_bindings WHERE workspace_id = OLD.id;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER extension_inputs_workspace_delete
AFTER DELETE ON workspaces
BEGIN
 DELETE FROM extension_inputs WHERE workspace_id = OLD.id;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER extension_installations_default_insert
AFTER INSERT ON extensions
BEGIN
 INSERT INTO extension_installations (extension_name, profile_id, workspace_id, created_at)
 VALUES (NEW.name, '', '', NEW.installed_at);
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER extension_installations_workspace_delete
AFTER DELETE ON workspaces
BEGIN
 DELETE FROM extension_installations WHERE workspace_id = OLD.id;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER extension_installations_workspace_insert
BEFORE INSERT ON extension_installations
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
 SELECT RAISE(ABORT, 'workspace_not_found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER extension_mcp_overrides_workspace_delete
AFTER DELETE ON workspaces
BEGIN
 DELETE FROM extension_mcp_overrides WHERE workspace_id = OLD.id;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER extensions_profile_enablement_delete
AFTER DELETE ON extensions
WHEN NOT EXISTS (
	SELECT 1 FROM extension_dev_links WHERE extension_name = OLD.name
)
BEGIN
	DELETE FROM extension_profile_enablement WHERE extension_name = OLD.name;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER gateway_ingress_resource_delete
AFTER DELETE ON resource_records
WHEN OLD.kind = 'automation.trigger'
BEGIN
	DELETE FROM gateway_ingress_bindings
	WHERE subject_id = OLD.id
		AND subject_kind = 'webhook_trigger';
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER gateway_ingress_trigger_resource_identity_update
AFTER UPDATE OF scope_kind, scope_id, spec_json ON resource_records
WHEN OLD.kind = 'automation.trigger' AND (
	OLD.scope_kind IS NOT NEW.scope_kind OR
	COALESCE(OLD.scope_id, '') <> COALESCE(NEW.scope_id, '') OR
	json_extract(OLD.spec_json, '$.event') IS NOT json_extract(NEW.spec_json, '$.event') OR
	json_extract(OLD.spec_json, '$.endpoint_slug') IS NOT json_extract(NEW.spec_json, '$.endpoint_slug') OR
	json_extract(OLD.spec_json, '$.webhook_id') IS NOT json_extract(NEW.spec_json, '$.webhook_id')
)
BEGIN
	DELETE FROM gateway_ingress_bindings
	WHERE subject_kind = 'webhook_trigger' AND subject_id = OLD.id;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER loop_runs_profile_owner_active BEFORE INSERT ON loop_runs BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER loop_runs_profile_owner_immutable BEFORE UPDATE OF profile_id ON loop_runs
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER profile_selections_workspace_delete AFTER DELETE ON workspaces BEGIN
	DELETE FROM profile_selections WHERE lens = 'workspace' AND workspace_id = OLD.id;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER session_health_workspace_insert_guard
BEFORE INSERT ON session_health
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER session_health_workspace_update_guard
BEFORE UPDATE OF workspace_id ON session_health
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER sessions_profile_owner_active BEFORE INSERT ON sessions BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER sessions_profile_owner_immutable BEFORE UPDATE OF profile_id ON sessions
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER sessions_workspace_insert_guard
BEFORE INSERT ON sessions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER sessions_workspace_update_guard
BEFORE UPDATE OF workspace_id ON sessions
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER tasks_profile_owner_active BEFORE INSERT ON tasks BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER tasks_profile_owner_immutable BEFORE UPDATE OF profile_id ON tasks
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER tool_approval_grants_workspace_insert_guard
BEFORE INSERT ON tool_approval_grants
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER tool_approval_grants_workspace_update_guard
BEFORE UPDATE OF workspace_id ON tool_approval_grants
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
	SELECT RAISE(ABORT, 'workspace not found');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER trg_sessions_archive_insert_guard
			BEFORE INSERT ON sessions
			WHEN NEW.archived_at IS NOT NULL AND NEW.state != 'stopped'
			BEGIN
				SELECT RAISE(ABORT, 'session is archived');
			END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER trg_sessions_archive_update_guard
			BEFORE UPDATE OF state, archived_at ON sessions
			WHEN NEW.archived_at IS NOT NULL AND NEW.state != 'stopped'
			BEGIN
				SELECT RAISE(ABORT, 'session is archived');
			END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER trg_task_runs_terminal_command_delete_guard
	BEFORE DELETE ON task_runs
	WHEN EXISTS (
		SELECT 1
		FROM task_run_terminal_commands
		WHERE run_id = OLD.id
	)
	BEGIN
		SELECT RAISE(ABORT, 'task run terminal command in progress');
	END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER trg_task_runs_terminal_command_guard
	BEFORE UPDATE ON task_runs
	WHEN EXISTS (
		SELECT 1
		FROM task_run_terminal_commands
		WHERE run_id = OLD.id
	)
	BEGIN
		SELECT RAISE(ABORT, 'task run terminal command in progress');
	END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER trg_tasks_terminal_command_delete_guard
	BEFORE DELETE ON tasks
	WHEN EXISTS (
		SELECT 1
		FROM task_run_terminal_commands
		WHERE task_id = OLD.id
	)
	BEGIN
		SELECT RAISE(ABORT, 'task run terminal command in progress');
	END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER workspace_deletion_intents_registration_guard
BEFORE INSERT ON workspaces
WHEN EXISTS (
    SELECT 1 FROM workspace_deletion_intents WHERE workspace_id = NEW.id
)
BEGIN
    SELECT RAISE(ABORT, 'workspace deletion pending');
END;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE TRIGGER workspace_scope_cleanup_after_delete
AFTER DELETE ON workspaces
BEGIN
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

PRAGMA foreign_keys = ON;
