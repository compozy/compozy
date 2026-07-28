-- +goose NO TRANSACTION
-- +goose Up
-- Destructive alpha hard cut approved by ADR-004/006/007/009 and the network-changes TechSpec.
-- Main migrations v2-v7 remain immutable; this v8 applies Network to their combined schema.
-- Retained execution owners are backfilled to the canonical Local snapshot. Peer-keyed durable
-- relations and implicit task-run coordination channels intentionally have no trustworthy mapping.
PRAGMA foreign_keys = OFF;

BEGIN IMMEDIATE;

ALTER TABLE sessions ADD COLUMN network_spec_json TEXT NOT NULL
  DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}'
  CHECK (json_valid(network_spec_json));
ALTER TABLE sessions ADD COLUMN network_mode TEXT NOT NULL DEFAULT 'local'
  CHECK (network_mode IN ('local', 'live'));
ALTER TABLE sessions ADD COLUMN network_channel TEXT;
ALTER TABLE sessions ADD COLUMN network_source TEXT NOT NULL DEFAULT 'built_in_local'
  CHECK (network_source IN (
    'explicit_request', 'task_profile', 'workspace_coordination',
    'loop_definition', 'automation_job', 'built_in_local'
  ));
UPDATE sessions
SET network_spec_json = '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}',
    network_mode = 'local',
    network_channel = NULL,
    network_source = 'built_in_local';
ALTER TABLE sessions DROP COLUMN channel;

ALTER TABLE loop_runs ADD COLUMN network_spec_json TEXT NOT NULL
  DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}'
  CHECK (json_valid(network_spec_json));
ALTER TABLE loop_runs ADD COLUMN network_mode TEXT NOT NULL DEFAULT 'local'
  CHECK (network_mode IN ('local', 'live'));
ALTER TABLE loop_runs ADD COLUMN network_channel TEXT;
ALTER TABLE loop_runs ADD COLUMN network_source TEXT NOT NULL DEFAULT 'built_in_local'
  CHECK (network_source IN (
    'explicit_request', 'task_profile', 'workspace_coordination',
    'loop_definition', 'automation_job', 'built_in_local'
  ));
UPDATE loop_runs
SET network_spec_json = '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}',
    network_mode = 'local',
    network_channel = NULL,
    network_source = 'built_in_local';

DROP INDEX idx_tasks_channel;
ALTER TABLE tasks DROP COLUMN network_channel;

ALTER TABLE task_execution_profiles ADD COLUMN network_mode TEXT NOT NULL DEFAULT ''
  CHECK (network_mode IN ('', 'local', 'live'));
ALTER TABLE task_execution_profiles ADD COLUMN network_channel_strategy TEXT
  CHECK (network_channel_strategy IS NULL OR network_channel_strategy IN ('named', 'run', 'loop_run'));
ALTER TABLE task_execution_profiles ADD COLUMN network_channel TEXT;
ALTER TABLE task_execution_profiles ADD COLUMN network_bounds_json TEXT
  CHECK (network_bounds_json IS NULL OR json_valid(network_bounds_json));

CREATE TABLE task_runs_new (
  id TEXT PRIMARY KEY,
  task_id TEXT REFERENCES tasks(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  attempt INTEGER NOT NULL CHECK (attempt > 0),
  previous_run_id TEXT,
  failure_kind TEXT NOT NULL DEFAULT '' CHECK (
    failure_kind = '' OR failure_kind IN ('operator_forced')
  ),
  claimed_by_kind TEXT CHECK (
    claimed_by_kind IS NULL OR claimed_by_kind IN (
      'human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon'
    )
  ),
  claimed_by_ref TEXT,
  session_id TEXT,
  origin_kind TEXT NOT NULL CHECK (
    origin_kind IN (
      'cli', 'web', 'uds', 'http', 'automation', 'extension', 'network', 'agent_session', 'daemon'
    )
  ),
  origin_ref TEXT NOT NULL,
  idempotency_key TEXT,
  network_spec_json TEXT NOT NULL
    DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}'
    CHECK (json_valid(network_spec_json)),
  network_mode TEXT NOT NULL DEFAULT 'local' CHECK (network_mode IN ('local', 'live')),
  network_channel TEXT,
  network_source TEXT NOT NULL DEFAULT 'built_in_local' CHECK (
    network_source IN (
      'explicit_request', 'task_profile', 'workspace_coordination',
      'loop_definition', 'automation_job', 'built_in_local'
    )
  ),
  designation_group_id TEXT NOT NULL DEFAULT '',
  queued_at TEXT NOT NULL,
  claimed_at TEXT,
  started_at TEXT,
  ended_at TEXT,
  error TEXT,
  metadata_json TEXT,
  result_json TEXT,
  summary TEXT NOT NULL DEFAULT '',
  claimed_agent_name TEXT NOT NULL DEFAULT '',
  claimed_peer_id TEXT NOT NULL DEFAULT '',
  terminalized_by_session_id TEXT NOT NULL DEFAULT '',
  terminalized_by_agent_name TEXT NOT NULL DEFAULT '',
  terminalized_by_peer_id TEXT NOT NULL DEFAULT '',
  terminalized_by_actor_kind TEXT NOT NULL DEFAULT '',
  terminalized_by_actor_ref TEXT NOT NULL DEFAULT '',
  review_required BOOLEAN NOT NULL DEFAULT 0 CHECK (review_required IN (0, 1)),
  review_request_round INTEGER NOT NULL DEFAULT 0 CHECK (review_request_round >= 0),
  review_policy_snapshot TEXT NOT NULL DEFAULT '' CHECK (
    review_policy_snapshot = '' OR review_policy_snapshot IN ('none', 'on_success', 'on_failure', 'always')
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
  run_kind TEXT NOT NULL DEFAULT 'worker'
    CHECK (run_kind IN ('worker', 'coordinator', 'network_wake')),
  loop_run_id TEXT,
  tokens_used INTEGER NOT NULL DEFAULT 0,
  network_wake_id TEXT,
  network_target_session_id TEXT,
  network_owner_key TEXT,
  CHECK (
    (claimed_by_kind IS NULL AND claimed_by_ref IS NULL) OR
    (claimed_by_kind IS NOT NULL AND claimed_by_ref IS NOT NULL)
  ),
  CHECK (status <> 'queued' OR session_id IS NULL),
  CHECK (run_kind = 'network_wake' OR task_id IS NOT NULL),
  CHECK (run_kind <> 'network_wake' OR task_id IS NULL),
  CHECK (
    (run_kind = 'network_wake' AND network_wake_id IS NOT NULL
      AND network_target_session_id IS NOT NULL AND network_owner_key IS NOT NULL) OR
    (run_kind <> 'network_wake' AND network_wake_id IS NULL
      AND network_target_session_id IS NULL AND network_owner_key IS NULL)
  )
);

INSERT INTO task_runs_new (
  id, task_id, status, attempt, previous_run_id, failure_kind, claimed_by_kind, claimed_by_ref,
  session_id, origin_kind, origin_ref, idempotency_key, network_spec_json, network_mode,
  network_channel, network_source, designation_group_id, queued_at, claimed_at, started_at,
  ended_at, error, metadata_json, result_json, summary, claimed_agent_name, claimed_peer_id,
  terminalized_by_session_id, terminalized_by_agent_name, terminalized_by_peer_id,
  terminalized_by_actor_kind, terminalized_by_actor_ref, review_required, review_request_round,
  review_policy_snapshot, review_request_id, parent_run_id, review_id, review_round,
  continuation_reason, missing_work_json, next_round_guidance, claim_token, claim_token_hash,
  lease_until, heartbeat_at, run_kind, loop_run_id, tokens_used,
  network_wake_id, network_target_session_id, network_owner_key
)
SELECT
  id, task_id, status, attempt, previous_run_id, failure_kind, claimed_by_kind, claimed_by_ref,
  session_id, origin_kind, origin_ref, idempotency_key,
  '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}',
  'local', NULL, 'built_in_local', designation_group_id, queued_at, claimed_at, started_at,
  ended_at, error, metadata_json, result_json, summary, claimed_agent_name, claimed_peer_id,
  terminalized_by_session_id, terminalized_by_agent_name, terminalized_by_peer_id,
  terminalized_by_actor_kind, terminalized_by_actor_ref, review_required, review_request_round,
  review_policy_snapshot, review_request_id, parent_run_id, review_id, review_round,
  continuation_reason, missing_work_json, next_round_guidance, claim_token, claim_token_hash,
  lease_until, heartbeat_at, run_kind, loop_run_id, tokens_used,
  NULL, NULL, NULL
FROM task_runs;

DROP TABLE task_runs;
ALTER TABLE task_runs_new RENAME TO task_runs;

CREATE INDEX idx_task_runs_active_lease_recovery
  ON task_runs(status, lease_until, heartbeat_at, id);
CREATE INDEX idx_task_runs_channel ON task_runs(network_channel);
CREATE INDEX idx_task_runs_designation_group ON task_runs(task_id, designation_group_id);
CREATE INDEX idx_task_runs_parent_run ON task_runs(parent_run_id);
CREATE INDEX idx_task_runs_pending_claim ON task_runs(status, lease_until, queued_at, id);
CREATE INDEX idx_task_runs_previous ON task_runs(previous_run_id);
CREATE INDEX idx_task_runs_review_request ON task_runs(review_request_id)
  WHERE review_request_id IS NOT NULL;
CREATE INDEX idx_task_runs_session ON task_runs(session_id);
CREATE INDEX idx_task_runs_session_status ON task_runs(session_id, status, lease_until);
CREATE INDEX idx_task_runs_target_session
  ON task_runs(network_target_session_id, status, queued_at, id)
  WHERE run_kind = 'network_wake';
CREATE INDEX idx_task_runs_status ON task_runs(status);
CREATE INDEX idx_task_runs_task ON task_runs(task_id, queued_at DESC, id DESC);
CREATE INDEX idx_task_runs_task_review_round ON task_runs(task_id, review_round)
  WHERE review_round > 0;
CREATE INDEX idx_task_runs_task_status ON task_runs(task_id, status, queued_at DESC, id DESC);
CREATE UNIQUE INDEX uq_task_runs_active_loop_coordinator ON task_runs(loop_run_id)
  WHERE run_kind = 'coordinator' AND status IN ('queued', 'claimed', 'starting', 'running');
CREATE UNIQUE INDEX uq_task_runs_review_id ON task_runs(review_id)
  WHERE review_id IS NOT NULL;

DROP TABLE network_delivery_guidance_state;
DROP TABLE network_thread_peer_token_stats;
DROP TABLE network_thread_participants;
DROP TABLE network_subscriptions;
DROP TABLE network_work;
DROP TABLE network_direct_rooms;
DROP TABLE network_channel_participants;

CREATE TABLE network_channel_participants (
  workspace_id TEXT NOT NULL,
  channel TEXT NOT NULL,
  session_id TEXT NOT NULL,
  PRIMARY KEY (workspace_id, channel, session_id),
  FOREIGN KEY (workspace_id, session_id)
    REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE network_direct_rooms (
  workspace_id TEXT NOT NULL,
  channel TEXT NOT NULL,
  direct_id TEXT NOT NULL,
  session_a TEXT NOT NULL,
  session_b TEXT NOT NULL,
  opened_at TEXT NOT NULL,
  last_activity_at TEXT NOT NULL,
  message_count INTEGER NOT NULL DEFAULT 0 CHECK (message_count >= 0),
  open_work_count INTEGER NOT NULL DEFAULT 0 CHECK (open_work_count >= 0),
  last_message_preview TEXT NOT NULL DEFAULT '',
  opened_sequence INTEGER NOT NULL DEFAULT 0,
  last_activity_sequence INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (workspace_id, channel, direct_id),
  UNIQUE (workspace_id, channel, session_a, session_b),
  CHECK (session_a < session_b),
  FOREIGN KEY (workspace_id, session_a)
    REFERENCES sessions(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, session_b)
    REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE network_subscriptions (
  workspace_id TEXT NOT NULL,
  channel TEXT NOT NULL,
  thread_id TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL,
  mode TEXT NOT NULL CHECK (mode IN ('mute', 'digest', 'full')),
  keyword_filters_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (workspace_id, channel, thread_id, session_id),
  FOREIGN KEY (workspace_id, channel)
    REFERENCES network_channels(workspace_id, channel) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, session_id)
    REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE network_thread_participants (
  workspace_id TEXT NOT NULL,
  channel TEXT NOT NULL,
  thread_id TEXT NOT NULL,
  session_id TEXT NOT NULL,
  first_message_id TEXT NOT NULL,
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  PRIMARY KEY (workspace_id, channel, thread_id, session_id),
  FOREIGN KEY (workspace_id, channel, thread_id)
    REFERENCES network_threads(workspace_id, channel, thread_id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, session_id)
    REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE network_thread_session_token_stats (
  workspace_id TEXT NOT NULL,
  channel TEXT NOT NULL,
  thread_id TEXT NOT NULL,
  session_id TEXT NOT NULL,
  delivered_count INTEGER NOT NULL DEFAULT 0 CHECK (delivered_count >= 0),
  prompt_size_bytes INTEGER NOT NULL DEFAULT 0 CHECK (prompt_size_bytes >= 0),
  estimated_prompt_tokens INTEGER NOT NULL DEFAULT 0 CHECK (estimated_prompt_tokens >= 0),
  first_delivered_at TEXT NOT NULL,
  last_delivered_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (workspace_id, channel, thread_id, session_id),
  FOREIGN KEY (workspace_id, channel, thread_id)
    REFERENCES network_threads(workspace_id, channel, thread_id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, session_id)
    REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE network_work (
  work_id TEXT NOT NULL,
  workspace_id TEXT NOT NULL,
  channel TEXT NOT NULL,
  surface TEXT NOT NULL CHECK (surface IN ('thread', 'direct')),
  thread_id TEXT,
  direct_id TEXT,
  opened_by_session_id TEXT NOT NULL,
  target_session_id TEXT,
  state TEXT NOT NULL CHECK (
    state IN ('submitted', 'working', 'needs_input', 'completed', 'failed', 'canceled')
  ),
  opened_at TEXT NOT NULL,
  last_activity_at TEXT NOT NULL,
  terminal_at TEXT,
  CHECK (
    (surface = 'thread' AND thread_id IS NOT NULL AND direct_id IS NULL) OR
    (surface = 'direct' AND direct_id IS NOT NULL AND thread_id IS NULL)
  ),
  PRIMARY KEY (workspace_id, work_id),
  FOREIGN KEY (workspace_id, channel, thread_id)
    REFERENCES network_threads(workspace_id, channel, thread_id) ON DELETE RESTRICT,
  FOREIGN KEY (workspace_id, channel, direct_id)
    REFERENCES network_direct_rooms(workspace_id, channel, direct_id) ON DELETE RESTRICT,
  FOREIGN KEY (workspace_id, opened_by_session_id)
    REFERENCES sessions(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, target_session_id)
    REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_network_channel_participants_session
  ON network_channel_participants(workspace_id, session_id, channel);
CREATE INDEX idx_network_direct_rooms_activity
  ON network_direct_rooms(workspace_id, channel, last_activity_sequence DESC, direct_id);
CREATE INDEX idx_network_direct_rooms_created
  ON network_direct_rooms(workspace_id, channel, opened_at, direct_id);
CREATE INDEX idx_network_direct_rooms_open_work
  ON network_direct_rooms(workspace_id, channel, open_work_count, last_activity_sequence DESC, direct_id);
CREATE INDEX idx_network_direct_rooms_session_a
  ON network_direct_rooms(workspace_id, channel, session_a, last_activity_sequence DESC);
CREATE INDEX idx_network_direct_rooms_session_b
  ON network_direct_rooms(workspace_id, channel, session_b, last_activity_sequence DESC);
CREATE INDEX idx_network_subscriptions_session
  ON network_subscriptions(workspace_id, session_id, channel, thread_id);
CREATE INDEX idx_network_thread_participants_session
  ON network_thread_participants(workspace_id, session_id, last_seen_at DESC);
CREATE INDEX idx_network_thread_session_token_stats_session
  ON network_thread_session_token_stats(workspace_id, channel, session_id, last_delivered_at DESC);
CREATE INDEX idx_network_work_conversation
  ON network_work(workspace_id, channel, surface, thread_id, direct_id, last_activity_at DESC);
CREATE INDEX idx_network_work_state
  ON network_work(workspace_id, state, last_activity_at DESC);

CREATE TABLE workspace_network_coordination (
  workspace_id TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
  enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
  revision INTEGER NOT NULL CHECK (revision >= 1),
  updated_at TEXT NOT NULL,
  updated_by TEXT NOT NULL CHECK (length(trim(updated_by)) > 0)
);

CREATE TABLE network_coordination_invitations (
  scope_kind TEXT NOT NULL CHECK (scope_kind IN ('workspace', 'task')),
  scope_id TEXT NOT NULL CHECK (length(trim(scope_id)) > 0),
  dismissed_at TEXT NOT NULL,
  dismissed_by TEXT NOT NULL CHECK (length(trim(dismissed_by)) > 0),
  PRIMARY KEY (scope_kind, scope_id)
);

CREATE TABLE network_availability (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
  epoch INTEGER NOT NULL CHECK (epoch >= 1),
  updated_at TEXT NOT NULL,
  updated_by TEXT NOT NULL CHECK (length(trim(updated_by)) > 0)
);
INSERT INTO network_availability (id, enabled, epoch, updated_at, updated_by)
VALUES (1, 1, 1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), 'migration:00004');

CREATE TABLE network_message_dispositions (
  message_id TEXT NOT NULL,
  recipient_session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  decision TEXT NOT NULL,
  decided_at TEXT NOT NULL,
  acceptance_seq INTEGER NOT NULL CHECK (acceptance_seq >= 1),
  PRIMARY KEY (message_id, recipient_session_id)
);

CREATE TABLE network_live_wakes (
  wake_id TEXT PRIMARY KEY,
  task_run_id TEXT NOT NULL UNIQUE REFERENCES task_runs(id) ON DELETE CASCADE,
  owner_key TEXT NOT NULL CHECK (length(trim(owner_key)) > 0),
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  channel TEXT NOT NULL,
  root_id TEXT NOT NULL,
  depth INTEGER NOT NULL CHECK (depth >= 0),
  state TEXT NOT NULL CHECK (state IN ('open', 'succeeded', 'failed', 'canceled', 'deadline_exceeded')),
  coalesce_until TEXT NOT NULL,
  reserved_wall_ms INTEGER NOT NULL CHECK (reserved_wall_ms > 0),
  actual_wall_ms INTEGER CHECK (actual_wall_ms IS NULL OR actual_wall_ms >= 0),
  reserved_at TEXT NOT NULL,
  settled_at TEXT,
  input_tokens INTEGER CHECK (input_tokens IS NULL OR input_tokens >= 0),
  output_tokens INTEGER CHECK (output_tokens IS NULL OR output_tokens >= 0),
  usage_state TEXT NOT NULL DEFAULT '' CHECK (usage_state IN ('', 'actual', 'usage_unavailable')),
  reason TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX uq_network_live_wakes_open_owner ON network_live_wakes(owner_key)
  WHERE state = 'open';
CREATE INDEX idx_network_live_wakes_workspace_channel
  ON network_live_wakes(workspace_id, channel, reserved_at DESC, wake_id);

CREATE TABLE network_wake_sources (
  owner_key TEXT NOT NULL,
  envelope_id TEXT NOT NULL,
  wake_id TEXT NOT NULL REFERENCES network_live_wakes(wake_id) ON DELETE CASCADE,
  PRIMARY KEY (owner_key, envelope_id)
);

CREATE TABLE network_participation_budgets (
  owner_key TEXT PRIMARY KEY CHECK (length(trim(owner_key)) > 0),
  wakes_used INTEGER NOT NULL DEFAULT 0 CHECK (wakes_used >= 0),
  wall_ms_used INTEGER NOT NULL DEFAULT 0 CHECK (wall_ms_used >= 0),
  input_tokens_used INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens_used >= 0),
  output_tokens_used INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens_used >= 0),
  exhausted_reason TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);

DELETE FROM network_channels WHERE purpose = 'task_run_coordination';

-- Atlas represents CHECK constraints added by SQLite ALTER TABLE differently from checks
-- inspected from the declarative schema. Rebuild the affected owners into the canonical shape
-- after the destructive cut so migration replay and fresh-schema creation remain equivalent.
DROP TRIGGER automation_watch_events_after_insert;
DROP TRIGGER automation_watch_events_after_terminal_update;

-- create "new_loop_runs" table
CREATE TABLE `new_loop_runs` (`id` text NULL, `workspace_id` text NOT NULL, `loop_name` text NOT NULL, `status` text NOT NULL, `generation` integer NOT NULL DEFAULT 0, `reattempt_strategy` text NOT NULL DEFAULT 'failed_only', `last_progress_at` timestamp NOT NULL, `consecutive_failures` integer NOT NULL DEFAULT 0, `budget_tokens` integer NOT NULL DEFAULT 0, `budget_wall_sec` integer NOT NULL DEFAULT 0, `budget_on_exceeded` text NOT NULL DEFAULT 'halt', `tokens_used` integer NOT NULL DEFAULT 0, `parent_loop_run_id` text NULL, `pause_requested` integer NOT NULL DEFAULT 0, `inputs_json` text NOT NULL, `created_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `iteration_cap` integer NOT NULL DEFAULT 0, `started_by_kind` text NOT NULL DEFAULT '', `started_by_ref` text NOT NULL DEFAULT '', `started_origin_kind` text NOT NULL DEFAULT '', `started_origin_ref` text NOT NULL DEFAULT '', `started_at` text NOT NULL DEFAULT '1970-01-01T00:00:00.000000000Z', `definition_version` integer NOT NULL DEFAULT 0, `definition_digest` text NOT NULL DEFAULT '', `active_gate_id` text NOT NULL DEFAULT '', `active_human_criteria_json` text NOT NULL DEFAULT '[]', `budget_approval_seq` integer NOT NULL DEFAULT 0, `start_metadata_json` text NOT NULL DEFAULT '{}', `origin_kind` text NOT NULL DEFAULT 'catalog', `origin_session_id` text NULL, `goal_cleared_at` timestamp NULL, `budget_version` integer NOT NULL DEFAULT 0, `goal_context_nudge_ratio` real NOT NULL DEFAULT 0.8, `control_actor_kind` text NULL, `control_actor_id` text NULL, `control_requested_at` timestamp NULL, `origin_creation_profile_ref` text NULL, `origin_policy_spec_digest` text NULL, `origin_creation_digest` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', PRIMARY KEY (`id`), CHECK (budget_version >= 0), CHECK (goal_context_nudge_ratio >= 0.0 AND goal_context_nudge_ratio <= 1.0), CHECK (origin_creation_profile_ref IS NULL OR length(trim(origin_creation_profile_ref)) > 0), CHECK (origin_policy_spec_digest IS NULL OR length(trim(origin_policy_spec_digest)) > 0), CHECK (origin_creation_digest IS NULL OR length(trim(origin_creation_digest)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
					'explicit_request', 'task_profile', 'workspace_coordination',
					'loop_definition', 'automation_job', 'built_in_local'
				)));
-- copy rows from old table "loop_runs" to new temporary table "new_loop_runs"
INSERT INTO `new_loop_runs` (`id`, `workspace_id`, `loop_name`, `status`, `generation`, `reattempt_strategy`, `last_progress_at`, `consecutive_failures`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`) SELECT `id`, `workspace_id`, `loop_name`, `status`, `generation`, `reattempt_strategy`, `last_progress_at`, `consecutive_failures`, `budget_tokens`, `budget_wall_sec`, `budget_on_exceeded`, `tokens_used`, `parent_loop_run_id`, `pause_requested`, `inputs_json`, `created_at`, `iteration_cap`, `started_by_kind`, `started_by_ref`, `started_origin_kind`, `started_origin_ref`, `started_at`, `definition_version`, `definition_digest`, `active_gate_id`, `active_human_criteria_json`, `budget_approval_seq`, `start_metadata_json`, `origin_kind`, `origin_session_id`, `goal_cleared_at`, `budget_version`, `goal_context_nudge_ratio`, `control_actor_kind`, `control_actor_id`, `control_requested_at`, `origin_creation_profile_ref`, `origin_policy_spec_digest`, `origin_creation_digest`, `network_spec_json`, `network_mode`, `network_channel`, `network_source` FROM `loop_runs`;
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
			  AND status IN ('queued','running','watching','needs-approval','paused');
-- create "new_sessions" table
CREATE TABLE `new_sessions` (`id` text NULL, `name` text NULL, `agent_name` text NOT NULL, `provider` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL, `session_type` text NOT NULL DEFAULT 'user', `state` text NOT NULL, `acp_session_id` text NULL, `stop_reason` text NULL, `stop_detail` text NULL, `subprocess_pid` integer NOT NULL DEFAULT 0, `subprocess_started_at` text NULL, `last_update_at` text NULL, `stall_state` text NOT NULL DEFAULT '', `stall_reason` text NOT NULL DEFAULT '', `activity_json` text NOT NULL DEFAULT '', `attached_to` text NOT NULL DEFAULT '', `attach_expires_at` text NULL, `transcript_epoch` integer NOT NULL DEFAULT 0, `sandbox_id` text NOT NULL DEFAULT '', `sandbox_backend` text NOT NULL DEFAULT 'local', `sandbox_profile` text NOT NULL DEFAULT '', `sandbox_instance_id` text NOT NULL DEFAULT '', `sandbox_state` text NOT NULL DEFAULT '', `sandbox_provider_state_json` text NOT NULL DEFAULT '', `sandbox_last_sync_at` text NULL, `sandbox_last_sync_error` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `failure_kind` text NULL, `failure_summary` text NOT NULL DEFAULT '', `crash_bundle_path` text NOT NULL DEFAULT '', `parent_session_id` text NULL, `root_session_id` text NULL, `spawn_depth` integer NOT NULL DEFAULT 0, `spawn_role` text NULL, `ttl_expires_at` text NULL, `auto_stop_on_parent` boolean NOT NULL DEFAULT 0, `spawn_budget_json` text NOT NULL DEFAULT '{}', `permission_policy_json` text NOT NULL DEFAULT '{}', `soul_snapshot_id` text NULL, `soul_digest` text NOT NULL DEFAULT '', `parent_soul_digest` text NOT NULL DEFAULT '', `input_generation` integer NOT NULL DEFAULT 0, `creation_digest` text NULL, `policy_spec_digest` text NULL, `creation_profile_ref` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', PRIMARY KEY (`id`), UNIQUE (`workspace_id`, `id`), CONSTRAINT `0` FOREIGN KEY (`soul_snapshot_id`) REFERENCES `agent_soul_snapshots` (`id`) ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT `1` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (creation_digest IS NULL OR length(trim(creation_digest)) > 0), CHECK (policy_spec_digest IS NULL OR length(trim(policy_spec_digest)) > 0), CHECK (creation_profile_ref IS NULL OR length(trim(creation_profile_ref)) > 0), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (network_source IN (
					'explicit_request', 'task_profile', 'workspace_coordination',
					'loop_definition', 'automation_job', 'built_in_local'
				)));
-- copy rows from old table "sessions" to new temporary table "new_sessions"
INSERT INTO `new_sessions` (`id`, `name`, `agent_name`, `provider`, `workspace_id`, `session_type`, `state`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`) SELECT `id`, `name`, `agent_name`, `provider`, `workspace_id`, `session_type`, `state`, `acp_session_id`, `stop_reason`, `stop_detail`, `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, `stall_reason`, `activity_json`, `attached_to`, `attach_expires_at`, `transcript_epoch`, `sandbox_id`, `sandbox_backend`, `sandbox_profile`, `sandbox_instance_id`, `sandbox_state`, `sandbox_provider_state_json`, `sandbox_last_sync_at`, `sandbox_last_sync_error`, `created_at`, `updated_at`, `failure_kind`, `failure_summary`, `crash_bundle_path`, `parent_session_id`, `root_session_id`, `spawn_depth`, `spawn_role`, `ttl_expires_at`, `auto_stop_on_parent`, `spawn_budget_json`, `permission_policy_json`, `soul_snapshot_id`, `soul_digest`, `parent_soul_digest`, `input_generation`, `creation_digest`, `policy_spec_digest`, `creation_profile_ref`, `network_spec_json`, `network_mode`, `network_channel`, `network_source` FROM `sessions`;
-- drop "sessions" table after copying rows
DROP TABLE `sessions`;
-- rename temporary table "new_sessions" to "sessions"
ALTER TABLE `new_sessions` RENAME TO `sessions`;
-- create index "idx_sessions_attach_lock" to table: "sessions"
CREATE INDEX `idx_sessions_attach_lock` ON `sessions` (`attached_to`, `attach_expires_at`);
-- create index "idx_sessions_catalog_activity" to table: "sessions"
CREATE INDEX idx_sessions_catalog_activity
  ON sessions(
    workspace_id, state, COALESCE(last_update_at, updated_at) DESC,
    updated_at DESC, created_at DESC, id DESC
  );
-- create index "idx_sessions_catalog_recent" to table: "sessions"
CREATE INDEX `idx_sessions_catalog_recent` ON `sessions` (`workspace_id`, `state`, `updated_at` DESC, `created_at` DESC, `id` DESC);
-- create index "idx_sessions_parent" to table: "sessions"
CREATE INDEX `idx_sessions_parent` ON `sessions` (`parent_session_id`);
-- create index "idx_sessions_resumable" to table: "sessions"
CREATE INDEX `idx_sessions_resumable` ON `sessions` (`state`, `failure_kind`, `last_update_at`, `updated_at`);
-- create index "idx_sessions_root" to table: "sessions"
CREATE INDEX `idx_sessions_root` ON `sessions` (`root_session_id`);
-- create index "idx_sessions_soul_snapshot" to table: "sessions"
CREATE INDEX `idx_sessions_soul_snapshot` ON `sessions` (`soul_snapshot_id`);
-- create index "idx_sessions_spawn_role" to table: "sessions"
CREATE INDEX `idx_sessions_spawn_role` ON `sessions` (`spawn_role`);
-- create index "idx_sessions_type_depth" to table: "sessions"
CREATE INDEX `idx_sessions_type_depth` ON `sessions` (`session_type`, `spawn_depth`);
-- create "new_task_execution_profiles" table
CREATE TABLE `new_task_execution_profiles` (`task_id` text NULL, `coordinator_mode` text NOT NULL DEFAULT 'inherit', `coordinator_agent_name` text NOT NULL DEFAULT '', `coordinator_provider` text NOT NULL DEFAULT '', `coordinator_model` text NOT NULL DEFAULT '', `coordinator_guidance` text NOT NULL DEFAULT '', `worker_mode` text NOT NULL DEFAULT 'inherit', `worker_agent_name` text NOT NULL DEFAULT '', `worker_provider` text NOT NULL DEFAULT '', `worker_model` text NOT NULL DEFAULT '', `review_agent_name` text NOT NULL DEFAULT '', `review_provider` text NOT NULL DEFAULT '', `review_model` text NOT NULL DEFAULT '', `sandbox_mode` text NOT NULL DEFAULT 'inherit', `sandbox_ref` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `updated_at` text NOT NULL, `runtime_mode` text NOT NULL DEFAULT 'default', `network_mode` text NOT NULL DEFAULT '', `network_channel_strategy` text NULL, `network_channel` text NULL, `network_bounds_json` text NULL, PRIMARY KEY (`task_id`), CONSTRAINT `0` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (
				coordinator_mode IN ('inherit', 'guided')
			), CHECK (
				worker_mode IN ('inherit', 'select')
			), CHECK (
				sandbox_mode IN ('inherit', 'none', 'ref')
			), CHECK (
				runtime_mode IN ('default', 'evidence')
			), CHECK (network_mode IN ('', 'local', 'live')), CHECK (
				network_channel_strategy IS NULL OR network_channel_strategy IN ('named', 'run', 'loop_run')
			), CHECK (
				network_bounds_json IS NULL OR json_valid(network_bounds_json)
			), CHECK (
				(sandbox_mode = 'ref' AND sandbox_ref <> '') OR
				(sandbox_mode <> 'ref' AND sandbox_ref = '')
			));
-- copy rows from old table "task_execution_profiles" to new temporary table "new_task_execution_profiles"
INSERT INTO `new_task_execution_profiles` (`task_id`, `coordinator_mode`, `coordinator_agent_name`, `coordinator_provider`, `coordinator_model`, `coordinator_guidance`, `worker_mode`, `worker_agent_name`, `worker_provider`, `worker_model`, `review_agent_name`, `review_provider`, `review_model`, `sandbox_mode`, `sandbox_ref`, `created_at`, `updated_at`, `runtime_mode`, `network_mode`, `network_channel_strategy`, `network_channel`, `network_bounds_json`) SELECT `task_id`, `coordinator_mode`, `coordinator_agent_name`, `coordinator_provider`, `coordinator_model`, `coordinator_guidance`, `worker_mode`, `worker_agent_name`, `worker_provider`, `worker_model`, `review_agent_name`, `review_provider`, `review_model`, `sandbox_mode`, `sandbox_ref`, `created_at`, `updated_at`, `runtime_mode`, `network_mode`, `network_channel_strategy`, `network_channel`, `network_bounds_json` FROM `task_execution_profiles`;
-- drop "task_execution_profiles" table after copying rows
DROP TABLE `task_execution_profiles`;
-- rename temporary table "new_task_execution_profiles" to "task_execution_profiles"
ALTER TABLE `new_task_execution_profiles` RENAME TO `task_execution_profiles`;
-- create index "task_execution_profiles_task_id_idx" to table: "task_execution_profiles"
CREATE INDEX `task_execution_profiles_task_id_idx` ON `task_execution_profiles` (`task_id`);
-- create "new_task_runs" table
CREATE TABLE `new_task_runs` (`id` text NULL, `task_id` text NULL, `status` text NOT NULL, `attempt` integer NOT NULL, `previous_run_id` text NULL, `failure_kind` text NOT NULL DEFAULT '', `claimed_by_kind` text NULL, `claimed_by_ref` text NULL, `session_id` text NULL, `origin_kind` text NOT NULL, `origin_ref` text NOT NULL, `idempotency_key` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `designation_group_id` text NOT NULL DEFAULT '', `queued_at` text NOT NULL, `claimed_at` text NULL, `started_at` text NULL, `ended_at` text NULL, `error` text NULL, `metadata_json` text NULL, `result_json` text NULL, `summary` text NOT NULL DEFAULT '', `claimed_agent_name` text NOT NULL DEFAULT '', `claimed_peer_id` text NOT NULL DEFAULT '', `terminalized_by_session_id` text NOT NULL DEFAULT '', `terminalized_by_agent_name` text NOT NULL DEFAULT '', `terminalized_by_peer_id` text NOT NULL DEFAULT '', `terminalized_by_actor_kind` text NOT NULL DEFAULT '', `terminalized_by_actor_ref` text NOT NULL DEFAULT '', `review_required` boolean NOT NULL DEFAULT 0, `review_request_round` integer NOT NULL DEFAULT 0, `review_policy_snapshot` text NOT NULL DEFAULT '', `review_request_id` text NULL, `parent_run_id` text NULL, `review_id` text NULL, `review_round` integer NOT NULL DEFAULT 0, `continuation_reason` text NOT NULL DEFAULT '', `missing_work_json` text NOT NULL DEFAULT '[]', `next_round_guidance` text NOT NULL DEFAULT '', `claim_token` text NULL, `claim_token_hash` text NULL, `lease_until` text NULL, `heartbeat_at` text NULL, `run_kind` text NOT NULL DEFAULT 'worker', `loop_run_id` text NULL, `tokens_used` integer NOT NULL DEFAULT 0, `network_wake_id` text NULL, `network_target_session_id` text NULL, `network_owner_key` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`review_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `1` FOREIGN KEY (`parent_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `2` FOREIGN KEY (`review_request_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `3` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (tokens_used >= 0), CHECK (attempt > 0), CHECK (
			failure_kind = '' OR failure_kind IN ('operator_forced')
		), CHECK (
			claimed_by_kind IS NULL OR claimed_by_kind IN (
				'human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon'
			)
		), CHECK (
			origin_kind IN (
				'cli', 'web', 'uds', 'http', 'automation', 'extension', 'network', 'agent_session', 'daemon'
			)
		), CHECK (json_valid(network_spec_json)), CHECK (network_mode IN ('local', 'live')), CHECK (
			network_source IN (
				'explicit_request', 'task_profile', 'workspace_coordination',
				'loop_definition', 'automation_job', 'built_in_local'
			)
		), CHECK (review_required IN (0, 1)), CHECK (review_request_round >= 0), CHECK (
			review_policy_snapshot = '' OR
			review_policy_snapshot IN ('none', 'on_success', 'on_failure', 'always')
		), CHECK (review_round >= 0), CHECK (run_kind IN ('worker', 'coordinator', 'network_wake')), CHECK (
			(claimed_by_kind IS NULL AND claimed_by_ref IS NULL) OR
			(claimed_by_kind IS NOT NULL AND claimed_by_ref IS NOT NULL)
		), CHECK (status <> 'queued' OR session_id IS NULL), CHECK (run_kind = 'network_wake' OR task_id IS NOT NULL), CHECK (run_kind <> 'network_wake' OR task_id IS NULL), CHECK (
			(run_kind = 'network_wake' AND network_wake_id IS NOT NULL
				AND network_target_session_id IS NOT NULL AND network_owner_key IS NOT NULL) OR
			(run_kind <> 'network_wake' AND network_wake_id IS NULL
				AND network_target_session_id IS NULL AND network_owner_key IS NULL)
		));
-- copy rows from old table "task_runs" to new temporary table "new_task_runs"
INSERT INTO `new_task_runs` (`id`, `task_id`, `status`, `attempt`, `previous_run_id`, `failure_kind`, `claimed_by_kind`, `claimed_by_ref`, `session_id`, `origin_kind`, `origin_ref`, `idempotency_key`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `designation_group_id`, `queued_at`, `claimed_at`, `started_at`, `ended_at`, `error`, `metadata_json`, `result_json`, `summary`, `claimed_agent_name`, `claimed_peer_id`, `terminalized_by_session_id`, `terminalized_by_agent_name`, `terminalized_by_peer_id`, `terminalized_by_actor_kind`, `terminalized_by_actor_ref`, `review_required`, `review_request_round`, `review_policy_snapshot`, `review_request_id`, `parent_run_id`, `review_id`, `review_round`, `continuation_reason`, `missing_work_json`, `next_round_guidance`, `claim_token`, `claim_token_hash`, `lease_until`, `heartbeat_at`, `run_kind`, `loop_run_id`, `tokens_used`, `network_wake_id`, `network_target_session_id`, `network_owner_key`) SELECT `id`, `task_id`, `status`, `attempt`, `previous_run_id`, `failure_kind`, `claimed_by_kind`, `claimed_by_ref`, `session_id`, `origin_kind`, `origin_ref`, `idempotency_key`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`, `designation_group_id`, `queued_at`, `claimed_at`, `started_at`, `ended_at`, `error`, `metadata_json`, `result_json`, `summary`, `claimed_agent_name`, `claimed_peer_id`, `terminalized_by_session_id`, `terminalized_by_agent_name`, `terminalized_by_peer_id`, `terminalized_by_actor_kind`, `terminalized_by_actor_ref`, `review_required`, `review_request_round`, `review_policy_snapshot`, `review_request_id`, `parent_run_id`, `review_id`, `review_round`, `continuation_reason`, `missing_work_json`, `next_round_guidance`, `claim_token`, `claim_token_hash`, `lease_until`, `heartbeat_at`, `run_kind`, `loop_run_id`, `tokens_used`, `network_wake_id`, `network_target_session_id`, `network_owner_key` FROM `task_runs`;
-- drop "task_runs" table after copying rows
DROP TABLE `task_runs`;
-- rename temporary table "new_task_runs" to "task_runs"
ALTER TABLE `new_task_runs` RENAME TO `task_runs`;
-- create index "idx_task_runs_active_lease_recovery" to table: "task_runs"
CREATE INDEX `idx_task_runs_active_lease_recovery` ON `task_runs` (`status`, `lease_until`, `heartbeat_at`, `id`);
-- create index "idx_task_runs_channel" to table: "task_runs"
CREATE INDEX `idx_task_runs_channel` ON `task_runs` (`network_channel`);
-- create index "idx_task_runs_designation_group" to table: "task_runs"
CREATE INDEX `idx_task_runs_designation_group` ON `task_runs` (`task_id`, `designation_group_id`);
-- create index "idx_task_runs_parent_run" to table: "task_runs"
CREATE INDEX `idx_task_runs_parent_run` ON `task_runs` (`parent_run_id`);
-- create index "idx_task_runs_pending_claim" to table: "task_runs"
CREATE INDEX `idx_task_runs_pending_claim` ON `task_runs` (`status`, `lease_until`, `queued_at`, `id`);
-- create index "idx_task_runs_previous" to table: "task_runs"
CREATE INDEX `idx_task_runs_previous` ON `task_runs` (`previous_run_id`);
-- create index "idx_task_runs_review_request" to table: "task_runs"
CREATE INDEX `idx_task_runs_review_request` ON `task_runs` (`review_request_id`) WHERE review_request_id IS NOT NULL;
-- create index "idx_task_runs_session" to table: "task_runs"
CREATE INDEX `idx_task_runs_session` ON `task_runs` (`session_id`);
-- create index "idx_task_runs_session_status" to table: "task_runs"
CREATE INDEX `idx_task_runs_session_status` ON `task_runs` (`session_id`, `status`, `lease_until`);
-- create index "idx_task_runs_target_session" to table: "task_runs"
CREATE INDEX `idx_task_runs_target_session` ON `task_runs` (`network_target_session_id`, `status`, `queued_at`, `id`) WHERE run_kind = 'network_wake';
-- create index "idx_task_runs_status" to table: "task_runs"
CREATE INDEX `idx_task_runs_status` ON `task_runs` (`status`);
-- create index "idx_task_runs_task" to table: "task_runs"
CREATE INDEX `idx_task_runs_task` ON `task_runs` (`task_id`, `queued_at` DESC, `id` DESC);
-- create index "idx_task_runs_task_review_round" to table: "task_runs"
CREATE INDEX `idx_task_runs_task_review_round` ON `task_runs` (`task_id`, `review_round`) WHERE review_round > 0;
-- create index "idx_task_runs_task_status" to table: "task_runs"
CREATE INDEX `idx_task_runs_task_status` ON `task_runs` (`task_id`, `status`, `queued_at` DESC, `id` DESC);
-- create index "uq_task_runs_active_loop_coordinator" to table: "task_runs"
CREATE UNIQUE INDEX `uq_task_runs_active_loop_coordinator` ON `task_runs` (`loop_run_id`) WHERE run_kind = 'coordinator' AND status IN ('queued', 'claimed', 'starting', 'running');
-- create index "uq_task_runs_review_id" to table: "task_runs"
CREATE UNIQUE INDEX `uq_task_runs_review_id` ON `task_runs` (`review_id`) WHERE review_id IS NOT NULL;
-- create "new_network_work" table
CREATE TABLE `new_network_work` (`work_id` text NOT NULL, `workspace_id` text NOT NULL, `channel` text NOT NULL, `surface` text NOT NULL, `thread_id` text NULL, `direct_id` text NULL, `opened_by_session_id` text NOT NULL, `target_session_id` text NULL, `state` text NOT NULL, `opened_at` text NOT NULL, `last_activity_at` text NOT NULL, `terminal_at` text NULL, PRIMARY KEY (`work_id`, `workspace_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `channel`, `direct_id`) REFERENCES `network_direct_rooms` (`workspace_id`, `channel`, `direct_id`) ON UPDATE NO ACTION ON DELETE RESTRICT, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `channel`, `thread_id`) REFERENCES `network_threads` (`workspace_id`, `channel`, `thread_id`) ON UPDATE NO ACTION ON DELETE RESTRICT, CONSTRAINT `2` FOREIGN KEY (`workspace_id`, `target_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `3` FOREIGN KEY (`workspace_id`, `opened_by_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (surface IN ('thread', 'direct')), CHECK (
			state IN ('submitted', 'working', 'needs_input', 'completed', 'failed', 'canceled')
		), CHECK (
			(surface = 'thread' AND thread_id IS NOT NULL AND direct_id IS NULL)
			OR (surface = 'direct' AND direct_id IS NOT NULL AND thread_id IS NULL)
		));
-- copy rows from old table "network_work" to new temporary table "new_network_work"
INSERT INTO `new_network_work` (`work_id`, `workspace_id`, `channel`, `surface`, `thread_id`, `direct_id`, `opened_by_session_id`, `target_session_id`, `state`, `opened_at`, `last_activity_at`, `terminal_at`) SELECT `work_id`, `workspace_id`, `channel`, `surface`, `thread_id`, `direct_id`, `opened_by_session_id`, `target_session_id`, `state`, `opened_at`, `last_activity_at`, `terminal_at` FROM `network_work`;
-- drop "network_work" table after copying rows
DROP TABLE `network_work`;
-- rename temporary table "new_network_work" to "network_work"
ALTER TABLE `new_network_work` RENAME TO `network_work`;
-- create index "idx_network_work_conversation" to table: "network_work"
CREATE INDEX `idx_network_work_conversation` ON `network_work` (`workspace_id`, `channel`, `surface`, `thread_id`, `direct_id`, `last_activity_at` DESC);
-- create index "idx_network_work_state" to table: "network_work"
CREATE INDEX `idx_network_work_state` ON `network_work` (`workspace_id`, `state`, `last_activity_at` DESC);

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

COMMIT;

PRAGMA foreign_key_check;
PRAGMA foreign_keys = ON;

-- +goose Down
-- The alpha hard cut discards peer-keyed relations and implicit channel intent. Reconstructing
-- those values would fabricate authority, so this migration is intentionally irreversible.
SELECT irreversible_network_participation_hard_cut
FROM network_participation_migration_downgrade_is_not_supported;

