-- name: InsertLoopRun :exec
INSERT INTO loop_runs (
  profile_id,
  id, workspace_id, loop_name, status, historical, generation, reattempt_strategy, created_at, started_at,
  last_progress_at, definition_version, definition_digest, active_gate_id,
  active_human_criteria_json, budget_approval_seq, start_metadata_json,
  iteration_cap, budget_tokens, budget_wall_sec,
  budget_on_exceeded, tokens_used, parent_loop_run_id, pause_requested, inputs_json,
  started_by_kind, started_by_ref, started_origin_kind, started_origin_ref,
  goal_context_nudge_ratio, origin_kind, origin_session_id,
  origin_creation_profile_ref, origin_policy_spec_digest, origin_creation_digest,
  network_spec_json, network_mode, network_channel, network_source
) VALUES (
  sqlc.arg(profile_id),
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(loop_name), sqlc.arg(status), sqlc.arg(historical),
  sqlc.arg(generation), sqlc.arg(reattempt_strategy), sqlc.arg(created_at), sqlc.arg(started_at),
  sqlc.arg(last_progress_at), sqlc.arg(definition_version), sqlc.arg(definition_digest),
  sqlc.arg(active_gate_id), sqlc.arg(active_human_criteria_json), sqlc.arg(budget_approval_seq),
  sqlc.arg(start_metadata_json), sqlc.arg(iteration_cap),
  sqlc.arg(budget_tokens), sqlc.arg(budget_wall_sec), sqlc.arg(budget_on_exceeded),
  sqlc.arg(tokens_used), sqlc.narg(parent_loop_run_id), sqlc.arg(pause_requested),
  sqlc.arg(inputs_json), sqlc.arg(started_by_kind), sqlc.arg(started_by_ref),
  sqlc.arg(started_origin_kind), sqlc.arg(started_origin_ref), sqlc.arg(goal_context_nudge_ratio),
  sqlc.arg(origin_kind), sqlc.narg(origin_session_id), sqlc.narg(origin_creation_profile_ref),
  sqlc.narg(origin_policy_spec_digest), sqlc.narg(origin_creation_digest),
  sqlc.arg(network_spec_json), sqlc.arg(network_mode), sqlc.narg(network_channel),
  sqlc.arg(network_source)
);

-- name: GetLoopRun :one
SELECT * FROM loop_runs WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id);

-- name: GetLoopRunByID :one
SELECT * FROM loop_runs WHERE id = sqlc.arg(id);

-- name: FindActiveLoopRun :one
SELECT * FROM loop_runs
WHERE workspace_id = sqlc.arg(workspace_id)
  AND loop_name = sqlc.arg(loop_name)
  AND historical = 0
  AND status IN ('queued', 'running', 'watching', 'needs-approval', 'paused')
ORDER BY created_at ASC, id ASC
LIMIT 1;

-- name: FindLiveSessionGoal :one
SELECT * FROM loop_runs
WHERE workspace_id = sqlc.arg(workspace_id)
  AND origin_kind = 'session'
  AND historical = 0
  AND origin_session_id = sqlc.arg(origin_session_id)
  AND status IN ('queued', 'running', 'watching', 'needs-approval', 'paused')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: FindNewestSessionGoal :one
SELECT id, goal_cleared_at
FROM loop_runs
WHERE workspace_id = sqlc.arg(workspace_id)
  AND origin_kind = 'session'
  AND historical = 0
  AND origin_session_id = sqlc.arg(origin_session_id)
ORDER BY created_at DESC, rowid DESC
LIMIT 1;

-- name: MarkInlineGoalRunCleared :execrows
UPDATE loop_runs SET goal_cleared_at = sqlc.arg(goal_cleared_at)
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id) AND goal_cleared_at IS NULL;

-- name: GetInlineGoalOriginSession :one
SELECT workspace_id, state, creation_profile_ref, policy_spec_digest, creation_digest
FROM sessions WHERE id = sqlc.arg(session_id);

-- name: InsertLoopConfigIfMissing :exec
INSERT OR IGNORE INTO loop_config (
  workspace_id, loop_name, human_gate_enabled, reattempt_strategy, enabled_checks_json,
  iteration_cap, budget_tokens, budget_wall_sec, budget_on_exceeded,
  no_progress_window, fan_out_width, gate_max_revisions,
  runtime_defaults_json, runtime_rules_json, environment_json
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(loop_name), sqlc.arg(human_gate_enabled),
  sqlc.narg(reattempt_strategy), sqlc.arg(enabled_checks_json), sqlc.narg(iteration_cap),
  sqlc.narg(budget_tokens), sqlc.narg(budget_wall_sec), sqlc.narg(budget_on_exceeded),
  sqlc.narg(no_progress_window), sqlc.narg(fan_out_width), sqlc.narg(gate_max_revisions),
  sqlc.narg(runtime_defaults_json), sqlc.narg(runtime_rules_json), sqlc.narg(environment_json)
);

-- name: PatchLoopConfig :exec
UPDATE loop_config SET
  human_gate_enabled = CASE WHEN sqlc.arg(patch_human_gate) THEN sqlc.arg(human_gate_enabled) ELSE human_gate_enabled END,
  reattempt_strategy = CASE WHEN sqlc.arg(patch_reattempt) THEN sqlc.narg(reattempt_strategy) ELSE reattempt_strategy END,
  enabled_checks_json = CASE WHEN sqlc.arg(patch_enabled_checks) THEN sqlc.arg(enabled_checks_json) ELSE enabled_checks_json END,
  iteration_cap = CASE WHEN sqlc.arg(patch_iteration_cap) THEN sqlc.narg(iteration_cap) ELSE iteration_cap END,
  budget_tokens = CASE WHEN sqlc.arg(patch_budget_tokens) THEN sqlc.narg(budget_tokens) ELSE budget_tokens END,
  budget_wall_sec = CASE WHEN sqlc.arg(patch_budget_wall_sec) THEN sqlc.narg(budget_wall_sec) ELSE budget_wall_sec END,
  budget_on_exceeded = CASE WHEN sqlc.arg(patch_budget_on_exceeded) THEN sqlc.narg(budget_on_exceeded) ELSE budget_on_exceeded END,
  no_progress_window = CASE WHEN sqlc.arg(patch_no_progress_window) THEN sqlc.narg(no_progress_window) ELSE no_progress_window END,
  fan_out_width = CASE WHEN sqlc.arg(patch_fan_out_width) THEN sqlc.narg(fan_out_width) ELSE fan_out_width END,
  gate_max_revisions = CASE WHEN sqlc.arg(patch_gate_max_revisions) THEN sqlc.narg(gate_max_revisions) ELSE gate_max_revisions END,
  runtime_defaults_json = CASE WHEN sqlc.arg(patch_runtime_defaults) THEN sqlc.narg(runtime_defaults_json) ELSE runtime_defaults_json END,
  runtime_rules_json = CASE WHEN sqlc.arg(patch_runtime_rules) THEN sqlc.narg(runtime_rules_json) ELSE runtime_rules_json END,
  environment_json = CASE WHEN sqlc.arg(patch_environment) THEN sqlc.narg(environment_json) ELSE environment_json END
WHERE workspace_id = sqlc.arg(workspace_id) AND loop_name = sqlc.arg(loop_name);

-- name: GetLoopConfig :one
SELECT human_gate_enabled, reattempt_strategy, enabled_checks_json,
       iteration_cap, budget_tokens, budget_wall_sec, budget_on_exceeded,
       no_progress_window, fan_out_width, gate_max_revisions,
       runtime_defaults_json, runtime_rules_json, environment_json
FROM loop_config WHERE workspace_id = sqlc.arg(workspace_id) AND loop_name = sqlc.arg(loop_name);

-- name: DeleteLoopConfig :exec
DELETE FROM loop_config
WHERE workspace_id = sqlc.arg(workspace_id) AND loop_name = sqlc.arg(loop_name);

-- name: ListLoopRunEvents :many
SELECT watch_seq, id, loop_run_id, workspace_id, seq, kind, payload_json, at, delivery_key
FROM loop_run_events
WHERE workspace_id = sqlc.arg(workspace_id)
  AND loop_run_id = sqlc.arg(loop_run_id)
  AND seq > sqlc.arg(after_seq)
ORDER BY seq ASC
LIMIT sqlc.arg(row_limit);

-- name: GetLoopRunEventHead :one
SELECT CAST(COALESCE(MAX(seq), 0) AS INTEGER)
FROM loop_run_events
WHERE workspace_id = sqlc.arg(workspace_id)
  AND loop_run_id = sqlc.arg(loop_run_id);

-- name: ListLoopRunEventsBackward :many
SELECT watch_seq, id, loop_run_id, workspace_id, seq, kind, payload_json, at, delivery_key
FROM loop_run_events
WHERE workspace_id = sqlc.arg(workspace_id)
  AND loop_run_id = sqlc.arg(loop_run_id)
  AND seq <= sqlc.arg(fixed_head_seq)
  AND seq < sqlc.arg(before_seq)
ORDER BY seq DESC
LIMIT sqlc.arg(row_limit);

-- name: ListLoopRouteCauses :many
SELECT payload_json, at
FROM loop_run_events
WHERE workspace_id = sqlc.arg(workspace_id)
  AND loop_run_id = sqlc.arg(loop_run_id)
  AND kind = 'route_taken'
  AND CAST(json_extract(payload_json, '$.generation') AS INTEGER) = sqlc.arg(generation)
ORDER BY seq ASC;

-- name: InsertLoopRunEvent :exec
INSERT INTO loop_run_events (id, loop_run_id, workspace_id, seq, kind, payload_json, at)
VALUES (sqlc.arg(id), sqlc.arg(loop_run_id), sqlc.arg(workspace_id), sqlc.arg(seq),
        sqlc.arg(kind), sqlc.arg(payload_json), sqlc.arg(at));

-- name: NextLoopRunEventSequence :one
SELECT COALESCE(MAX(seq), 0) + 1 FROM loop_run_events WHERE loop_run_id = sqlc.arg(loop_run_id);

-- name: ListLoopUIAnnotations :many
SELECT node_id, x, y FROM loop_ui_annotations
WHERE workspace_id = sqlc.arg(workspace_id) AND loop_name = sqlc.arg(loop_name)
ORDER BY node_id ASC;

-- name: DeleteLoopUIAnnotations :exec
DELETE FROM loop_ui_annotations WHERE workspace_id = sqlc.arg(workspace_id) AND loop_name = sqlc.arg(loop_name);

-- name: InsertLoopUIAnnotation :exec
INSERT INTO loop_ui_annotations (workspace_id, loop_name, node_id, x, y)
VALUES (sqlc.arg(workspace_id), sqlc.arg(loop_name), sqlc.arg(node_id), sqlc.arg(x), sqlc.arg(y));
