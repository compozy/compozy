-- +goose NO TRANSACTION
-- +goose Up
-- Greenfield-alpha hard cut: workspace-qualify all Network participation evidence.
-- Any row whose authoritative workspace cannot be proven aborts this migration.
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = OFF;

BEGIN IMMEDIATE;

CREATE TEMP TABLE network_v6_assertions (
  invalid_rows INTEGER NOT NULL CHECK (invalid_rows = 0)
);

-- Every invitation must resolve to its explicit workspace. Global tasks cannot own
-- workspace-scoped invitation state.
INSERT INTO network_v6_assertions (invalid_rows)
SELECT COUNT(*)
FROM network_coordination_invitations AS invitation
WHERE (invitation.scope_kind = 'workspace' AND NOT EXISTS (
    SELECT 1 FROM workspaces AS workspace WHERE workspace.id = invitation.scope_id
  ))
  OR (invitation.scope_kind = 'task' AND NOT EXISTS (
    SELECT 1
    FROM tasks AS task
    WHERE task.id = invitation.scope_id AND task.workspace_id IS NOT NULL
  ));

-- A disposition inherits workspace only from its durable recipient, and its source
-- message must exist in the same workspace.
INSERT INTO network_v6_assertions (invalid_rows)
SELECT COUNT(*)
FROM network_message_dispositions AS disposition
LEFT JOIN sessions AS recipient ON recipient.id = disposition.recipient_session_id
WHERE recipient.id IS NULL
  OR NOT EXISTS (
    SELECT 1
    FROM network_timeline_log AS message
    WHERE message.workspace_id = recipient.workspace_id
      AND message.message_id = disposition.message_id
  );

-- Source rows inherit workspace and owner from the wake ledger. The source message
-- must resolve inside that same workspace.
INSERT INTO network_v6_assertions (invalid_rows)
SELECT COUNT(*)
FROM network_wake_sources AS source
LEFT JOIN network_live_wakes AS wake ON wake.wake_id = source.wake_id
WHERE wake.wake_id IS NULL
  OR wake.owner_key <> source.owner_key
  OR NOT EXISTS (
    SELECT 1
    FROM network_timeline_log AS message
    WHERE message.workspace_id = wake.workspace_id
      AND message.message_id = source.envelope_id
  );

-- An owner budget is valid only when its historical wake evidence proves exactly
-- one workspace.
INSERT INTO network_v6_assertions (invalid_rows)
SELECT COUNT(*)
FROM network_participation_budgets AS budget
WHERE (
  SELECT COUNT(DISTINCT wake.workspace_id)
  FROM network_live_wakes AS wake
  WHERE wake.owner_key = budget.owner_key
) <> 1;

-- A taskless wake run must agree with its ledger, target session, and immutable
-- participation snapshot. No default workspace is fabricated.
INSERT INTO network_v6_assertions (invalid_rows)
SELECT COUNT(*)
FROM task_runs AS run
LEFT JOIN network_live_wakes AS wake
  ON wake.task_run_id = run.id
  AND wake.wake_id = run.network_wake_id
  AND wake.owner_key = run.network_owner_key
LEFT JOIN sessions AS target ON target.id = run.network_target_session_id
WHERE run.run_kind = 'network_wake'
  AND (
    wake.wake_id IS NULL
    OR target.id IS NULL
    OR target.workspace_id <> wake.workspace_id
    OR COALESCE(json_extract(run.network_spec_json, '$.workspace_id'), '') <> wake.workspace_id
  );
-- create "new_network_coordination_invitations" table
CREATE TABLE `new_network_coordination_invitations` (`workspace_id` text NOT NULL, `scope_kind` text NOT NULL, `scope_id` text NOT NULL, `dismissed_at` text NOT NULL, `dismissed_by` text NOT NULL, PRIMARY KEY (`workspace_id`, `scope_kind`, `scope_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (scope_kind IN ('workspace', 'task')), CHECK (length(trim(scope_id)) > 0), CHECK (length(trim(dismissed_by)) > 0), CHECK (scope_kind <> 'workspace' OR scope_id = workspace_id));
-- copy rows from old table "network_coordination_invitations" to new temporary table "new_network_coordination_invitations"
INSERT INTO `new_network_coordination_invitations` (
  `workspace_id`, `scope_kind`, `scope_id`, `dismissed_at`, `dismissed_by`
)
SELECT
  CASE invitation.scope_kind
    WHEN 'workspace' THEN invitation.scope_id
    WHEN 'task' THEN (SELECT task.workspace_id FROM tasks AS task WHERE task.id = invitation.scope_id)
  END,
  invitation.scope_kind,
  invitation.scope_id,
  invitation.dismissed_at,
  invitation.dismissed_by
FROM network_coordination_invitations AS invitation;
-- drop "network_coordination_invitations" table after copying rows
DROP TABLE `network_coordination_invitations`;
-- rename temporary table "new_network_coordination_invitations" to "network_coordination_invitations"
ALTER TABLE `new_network_coordination_invitations` RENAME TO `network_coordination_invitations`;
-- create "new_network_message_dispositions" table
CREATE TABLE `new_network_message_dispositions` (`workspace_id` text NOT NULL, `message_id` text NOT NULL, `recipient_session_id` text NOT NULL, `decision` text NOT NULL, `decided_at` text NOT NULL, `acceptance_seq` integer NOT NULL, PRIMARY KEY (`workspace_id`, `message_id`, `recipient_session_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `message_id`) REFERENCES `network_timeline_log` (`workspace_id`, `message_id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `recipient_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (acceptance_seq >= 1));
-- copy rows from old table "network_message_dispositions" to new temporary table "new_network_message_dispositions"
INSERT INTO `new_network_message_dispositions` (
  `workspace_id`, `message_id`, `recipient_session_id`, `decision`, `decided_at`, `acceptance_seq`
)
SELECT
  (SELECT recipient.workspace_id FROM sessions AS recipient WHERE recipient.id = disposition.recipient_session_id),
  disposition.message_id,
  disposition.recipient_session_id,
  disposition.decision,
  disposition.decided_at,
  disposition.acceptance_seq
FROM network_message_dispositions AS disposition;
-- drop "network_message_dispositions" table after copying rows
DROP TABLE `network_message_dispositions`;
-- rename temporary table "new_network_message_dispositions" to "network_message_dispositions"
ALTER TABLE `new_network_message_dispositions` RENAME TO `network_message_dispositions`;
-- create "new_network_live_wakes" table
CREATE TABLE `new_network_live_wakes` (`wake_id` text NOT NULL, `task_run_id` text NOT NULL, `owner_key` text NOT NULL, `workspace_id` text NOT NULL, `channel` text NOT NULL, `root_id` text NOT NULL, `depth` integer NOT NULL, `state` text NOT NULL, `coalesce_until` text NOT NULL, `reserved_wall_ms` integer NOT NULL, `actual_wall_ms` integer NULL, `reserved_at` text NOT NULL, `settled_at` text NULL, `input_tokens` integer NULL, `output_tokens` integer NULL, `usage_state` text NOT NULL DEFAULT '', `reason` text NOT NULL DEFAULT '', PRIMARY KEY (`wake_id`, `workspace_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `task_run_id`) REFERENCES `task_runs` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`task_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(owner_key)) > 0), CHECK (depth >= 0), CHECK (state IN ('open', 'succeeded', 'failed', 'canceled', 'deadline_exceeded')), CHECK (reserved_wall_ms > 0), CHECK (actual_wall_ms IS NULL OR actual_wall_ms >= 0), CHECK (input_tokens IS NULL OR input_tokens >= 0), CHECK (output_tokens IS NULL OR output_tokens >= 0), CHECK (usage_state IN ('', 'actual', 'usage_unavailable')));
-- copy rows from old table "network_live_wakes" to new temporary table "new_network_live_wakes"
INSERT INTO `new_network_live_wakes` (`wake_id`, `task_run_id`, `owner_key`, `workspace_id`, `channel`, `root_id`, `depth`, `state`, `coalesce_until`, `reserved_wall_ms`, `actual_wall_ms`, `reserved_at`, `settled_at`, `input_tokens`, `output_tokens`, `usage_state`, `reason`) SELECT `wake_id`, `task_run_id`, `owner_key`, `workspace_id`, `channel`, `root_id`, `depth`, `state`, `coalesce_until`, `reserved_wall_ms`, `actual_wall_ms`, `reserved_at`, `settled_at`, `input_tokens`, `output_tokens`, `usage_state`, `reason` FROM `network_live_wakes`;
-- drop "network_live_wakes" table after copying rows
DROP TABLE `network_live_wakes`;
-- rename temporary table "new_network_live_wakes" to "network_live_wakes"
ALTER TABLE `new_network_live_wakes` RENAME TO `network_live_wakes`;
-- create index "network_live_wakes_task_run_id" to table: "network_live_wakes"
CREATE UNIQUE INDEX `network_live_wakes_task_run_id` ON `network_live_wakes` (`task_run_id`);
-- create index "network_live_wakes_workspace_id_wake_id_owner_key" to table: "network_live_wakes"
CREATE UNIQUE INDEX `network_live_wakes_workspace_id_wake_id_owner_key` ON `network_live_wakes` (`workspace_id`, `wake_id`, `owner_key`);
-- create index "uq_network_live_wakes_open_owner" to table: "network_live_wakes"
CREATE UNIQUE INDEX `uq_network_live_wakes_open_owner` ON `network_live_wakes` (`workspace_id`, `owner_key`) WHERE state = 'open';
-- create index "idx_network_live_wakes_workspace_channel" to table: "network_live_wakes"
CREATE INDEX `idx_network_live_wakes_workspace_channel` ON `network_live_wakes` (`workspace_id`, `channel`, `reserved_at` DESC, `wake_id`);
-- create "new_network_wake_sources" table
CREATE TABLE `new_network_wake_sources` (`workspace_id` text NOT NULL, `owner_key` text NOT NULL, `envelope_id` text NOT NULL, `wake_id` text NOT NULL, PRIMARY KEY (`workspace_id`, `owner_key`, `envelope_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `envelope_id`) REFERENCES `network_timeline_log` (`workspace_id`, `message_id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `wake_id`, `owner_key`) REFERENCES `network_live_wakes` (`workspace_id`, `wake_id`, `owner_key`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `2` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE);
-- copy rows from old table "network_wake_sources" to new temporary table "new_network_wake_sources"
INSERT INTO `new_network_wake_sources` (`workspace_id`, `owner_key`, `envelope_id`, `wake_id`)
SELECT
  (SELECT wake.workspace_id FROM network_live_wakes AS wake WHERE wake.wake_id = source.wake_id),
  source.owner_key,
  source.envelope_id,
  source.wake_id
FROM network_wake_sources AS source;
-- drop "network_wake_sources" table after copying rows
DROP TABLE `network_wake_sources`;
-- rename temporary table "new_network_wake_sources" to "network_wake_sources"
ALTER TABLE `new_network_wake_sources` RENAME TO `network_wake_sources`;
-- create "new_network_participation_budgets" table
CREATE TABLE `new_network_participation_budgets` (`workspace_id` text NOT NULL, `owner_key` text NOT NULL, `wakes_used` integer NOT NULL DEFAULT 0, `wall_ms_used` integer NOT NULL DEFAULT 0, `input_tokens_used` integer NOT NULL DEFAULT 0, `output_tokens_used` integer NOT NULL DEFAULT 0, `exhausted_reason` text NOT NULL DEFAULT '', `updated_at` text NOT NULL, PRIMARY KEY (`workspace_id`, `owner_key`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(owner_key)) > 0), CHECK (wakes_used >= 0), CHECK (wall_ms_used >= 0), CHECK (input_tokens_used >= 0), CHECK (output_tokens_used >= 0));
-- copy rows from old table "network_participation_budgets" to new temporary table "new_network_participation_budgets"
INSERT INTO `new_network_participation_budgets` (
  `workspace_id`, `owner_key`, `wakes_used`, `wall_ms_used`, `input_tokens_used`,
  `output_tokens_used`, `exhausted_reason`, `updated_at`
)
SELECT
  (SELECT MIN(wake.workspace_id) FROM network_live_wakes AS wake WHERE wake.owner_key = budget.owner_key),
  budget.owner_key,
  budget.wakes_used,
  budget.wall_ms_used,
  budget.input_tokens_used,
  budget.output_tokens_used,
  budget.exhausted_reason,
  budget.updated_at
FROM network_participation_budgets AS budget;
-- drop "network_participation_budgets" table after copying rows
DROP TABLE `network_participation_budgets`;
-- rename temporary table "new_network_participation_budgets" to "network_participation_budgets"
ALTER TABLE `new_network_participation_budgets` RENAME TO `network_participation_budgets`;
-- create "new_task_runs" table
CREATE TABLE `new_task_runs` (`id` text NULL, `task_id` text NULL, `workspace_id` text NULL, `status` text NOT NULL, `attempt` integer NOT NULL, `previous_run_id` text NULL, `failure_kind` text NOT NULL DEFAULT '', `claimed_by_kind` text NULL, `claimed_by_ref` text NULL, `session_id` text NULL, `origin_kind` text NOT NULL, `origin_ref` text NOT NULL, `idempotency_key` text NULL, `network_spec_json` text NOT NULL DEFAULT '{"version":"network-participation/v1","mode":"local","source":"built_in_local"}', `network_mode` text NOT NULL DEFAULT 'local', `network_channel` text NULL, `network_source` text NOT NULL DEFAULT 'built_in_local', `designation_group_id` text NOT NULL DEFAULT '', `queued_at` text NOT NULL, `claimed_at` text NULL, `started_at` text NULL, `ended_at` text NULL, `error` text NULL, `metadata_json` text NULL, `result_json` text NULL, `summary` text NOT NULL DEFAULT '', `claimed_agent_name` text NOT NULL DEFAULT '', `claimed_peer_id` text NOT NULL DEFAULT '', `terminalized_by_session_id` text NOT NULL DEFAULT '', `terminalized_by_agent_name` text NOT NULL DEFAULT '', `terminalized_by_peer_id` text NOT NULL DEFAULT '', `terminalized_by_actor_kind` text NOT NULL DEFAULT '', `terminalized_by_actor_ref` text NOT NULL DEFAULT '', `review_required` boolean NOT NULL DEFAULT 0, `review_request_round` integer NOT NULL DEFAULT 0, `review_policy_snapshot` text NOT NULL DEFAULT '', `review_request_id` text NULL, `parent_run_id` text NULL, `review_id` text NULL, `review_round` integer NOT NULL DEFAULT 0, `continuation_reason` text NOT NULL DEFAULT '', `missing_work_json` text NOT NULL DEFAULT '[]', `next_round_guidance` text NOT NULL DEFAULT '', `claim_token` text NULL, `claim_token_hash` text NULL, `lease_until` text NULL, `heartbeat_at` text NULL, `run_kind` text NOT NULL DEFAULT 'worker', `loop_run_id` text NULL, `tokens_used` integer NOT NULL DEFAULT 0, `network_wake_id` text NULL, `network_target_session_id` text NULL, `network_owner_key` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `network_target_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`review_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `2` FOREIGN KEY (`parent_run_id`) REFERENCES `task_runs` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `3` FOREIGN KEY (`review_request_id`) REFERENCES `task_run_reviews` (`review_id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT `4` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `5` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (attempt > 0), CHECK (
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
		), CHECK (review_round >= 0), CHECK (run_kind IN ('worker', 'coordinator', 'network_wake')), CHECK (tokens_used >= 0), CHECK (
			(claimed_by_kind IS NULL AND claimed_by_ref IS NULL) OR
			(claimed_by_kind IS NOT NULL AND claimed_by_ref IS NOT NULL)
		), CHECK (status <> 'queued' OR session_id IS NULL), CHECK (run_kind = 'network_wake' OR task_id IS NOT NULL), CHECK (run_kind <> 'network_wake' OR task_id IS NULL), CHECK (run_kind <> 'network_wake' OR workspace_id IS NOT NULL), CHECK (
			(run_kind = 'network_wake' AND network_wake_id IS NOT NULL
				AND network_target_session_id IS NOT NULL AND network_owner_key IS NOT NULL) OR
			(run_kind <> 'network_wake' AND network_wake_id IS NULL
				AND network_target_session_id IS NULL AND network_owner_key IS NULL)
		));
-- copy rows from old table "task_runs" to new temporary table "new_task_runs"
INSERT INTO `new_task_runs` (
  `id`, `task_id`, `workspace_id`, `status`, `attempt`, `previous_run_id`, `failure_kind`,
  `claimed_by_kind`, `claimed_by_ref`, `session_id`, `origin_kind`, `origin_ref`,
  `idempotency_key`, `network_spec_json`, `network_mode`, `network_channel`, `network_source`,
  `designation_group_id`, `queued_at`, `claimed_at`, `started_at`, `ended_at`, `error`,
  `metadata_json`, `result_json`, `summary`, `claimed_agent_name`, `claimed_peer_id`,
  `terminalized_by_session_id`, `terminalized_by_agent_name`, `terminalized_by_peer_id`,
  `terminalized_by_actor_kind`, `terminalized_by_actor_ref`, `review_required`,
  `review_request_round`, `review_policy_snapshot`, `review_request_id`, `parent_run_id`,
  `review_id`, `review_round`, `continuation_reason`, `missing_work_json`,
  `next_round_guidance`, `claim_token`, `claim_token_hash`, `lease_until`, `heartbeat_at`,
  `run_kind`, `loop_run_id`, `tokens_used`, `network_wake_id`, `network_target_session_id`,
  `network_owner_key`
)
SELECT
  run.id,
  run.task_id,
  CASE
    WHEN run.run_kind = 'network_wake' THEN (
      SELECT wake.workspace_id FROM network_live_wakes AS wake WHERE wake.task_run_id = run.id
    )
    ELSE (SELECT task.workspace_id FROM tasks AS task WHERE task.id = run.task_id)
  END,
  run.status,
  run.attempt,
  run.previous_run_id,
  run.failure_kind,
  run.claimed_by_kind,
  run.claimed_by_ref,
  run.session_id,
  run.origin_kind,
  run.origin_ref,
  run.idempotency_key,
  run.network_spec_json,
  run.network_mode,
  run.network_channel,
  run.network_source,
  run.designation_group_id,
  run.queued_at,
  run.claimed_at,
  run.started_at,
  run.ended_at,
  run.error,
  run.metadata_json,
  run.result_json,
  run.summary,
  run.claimed_agent_name,
  run.claimed_peer_id,
  run.terminalized_by_session_id,
  run.terminalized_by_agent_name,
  run.terminalized_by_peer_id,
  run.terminalized_by_actor_kind,
  run.terminalized_by_actor_ref,
  run.review_required,
  run.review_request_round,
  run.review_policy_snapshot,
  run.review_request_id,
  run.parent_run_id,
  run.review_id,
  run.review_round,
  run.continuation_reason,
  run.missing_work_json,
  run.next_round_guidance,
  run.claim_token,
  run.claim_token_hash,
  run.lease_until,
  run.heartbeat_at,
  run.run_kind,
  run.loop_run_id,
  run.tokens_used,
  run.network_wake_id,
  run.network_target_session_id,
  run.network_owner_key
FROM task_runs AS run;
-- drop "task_runs" table after copying rows
DROP TABLE `task_runs`;
-- rename temporary table "new_task_runs" to "task_runs"
ALTER TABLE `new_task_runs` RENAME TO `task_runs`;
-- create index "task_runs_workspace_id_id" to table: "task_runs"
CREATE UNIQUE INDEX `task_runs_workspace_id_id` ON `task_runs` (`workspace_id`, `id`);
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
-- create append-only Network wake lifecycle ledger after the workspace-qualified
-- task run and wake identities exist.
CREATE TABLE `network_wake_events` (
  `sequence` integer PRIMARY KEY AUTOINCREMENT,
  `workspace_id` text NOT NULL,
  `wake_id` text NOT NULL,
  `task_run_id` text NOT NULL,
  `owner_key` text NOT NULL,
  `target_session_id` text NOT NULL,
  `event_type` text NOT NULL,
  `state` text NOT NULL,
  `claim_token_hash` text NOT NULL DEFAULT '',
  `usage_state` text NOT NULL DEFAULT '',
  `actual_wall_ms` integer NULL,
  `input_tokens` integer NULL,
  `output_tokens` integer NULL,
  `reason` text NOT NULL DEFAULT '',
  `actor_kind` text NOT NULL,
  `actor_ref` text NOT NULL,
  `timestamp` text NOT NULL,
  CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `wake_id`, `owner_key`)
    REFERENCES `network_live_wakes` (`workspace_id`, `wake_id`, `owner_key`) ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `task_run_id`)
    REFERENCES `task_runs` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT `2` FOREIGN KEY (`workspace_id`, `target_session_id`)
    REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT `3` FOREIGN KEY (`workspace_id`)
    REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE,
  CHECK (length(trim(owner_key)) > 0),
  CHECK (event_type IN ('admitted', 'claimed', 'heartbeat', 'released', 'settled')),
  CHECK (usage_state IN ('', 'actual', 'usage_unavailable')),
  CHECK (actual_wall_ms IS NULL OR actual_wall_ms >= 0),
  CHECK (input_tokens IS NULL OR input_tokens >= 0),
  CHECK (output_tokens IS NULL OR output_tokens >= 0)
);
CREATE INDEX `idx_network_wake_events_wake_sequence`
  ON `network_wake_events` (`workspace_id`, `wake_id`, `sequence`);
DROP TABLE network_v6_assertions;

COMMIT;

PRAGMA foreign_key_check;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = ON;

-- +goose Down
-- Workspace-qualified Network evidence cannot be collapsed back to global keys
-- without fabricating ownership. This greenfield-alpha hard cut is irreversible.
SELECT irreversible_network_participation_hardening_downgrade_is_not_supported
FROM network_participation_hardening_migration_downgrade_is_not_supported;
