-- name: InsertTaskRunTerminalCommand :execrows
INSERT INTO task_run_terminal_commands (
    command_id,
    run_id,
    task_id,
    workspace_id,
    kind,
    phase,
    source_status,
    source_session_id,
    source_claim_token_hash,
    source_lease_until,
    intent_json,
    actor_json,
    command_at,
    admitted_at,
    updated_at
)
SELECT
    sqlc.arg(command_id),
    task_runs.id,
    task_runs.task_id,
    COALESCE(task_runs.workspace_id, ''),
    sqlc.arg(kind),
    sqlc.arg(phase),
    sqlc.arg(source_status),
    sqlc.arg(source_session_id),
    sqlc.arg(source_claim_token_hash),
    sqlc.narg(source_lease_until),
    sqlc.arg(intent_json),
    sqlc.arg(actor_json),
    sqlc.arg(command_at),
    sqlc.arg(admitted_at),
    sqlc.arg(updated_at)
FROM task_runs
WHERE task_runs.id = sqlc.arg(run_id)
  AND task_runs.task_id = sqlc.arg(task_id)
  AND COALESCE(task_runs.workspace_id, '') = sqlc.arg(workspace_id)
  AND task_runs.status = sqlc.arg(source_status)
  AND COALESCE(task_runs.session_id, '') = sqlc.arg(source_session_id)
  AND COALESCE(task_runs.claim_token_hash, '') = sqlc.arg(source_claim_token_hash)
  AND COALESCE(task_runs.lease_until, '') = COALESCE(sqlc.narg(source_lease_until), '')
ON CONFLICT(run_id) DO NOTHING;

-- name: GetTaskRunTerminalCommandByScope :one
SELECT
    command_id,
    run_id,
    task_id,
    workspace_id,
    kind,
    phase,
    source_status,
    source_session_id,
    source_claim_token_hash,
    source_lease_until,
    intent_json,
    actor_json,
    command_at,
    admitted_at,
    updated_at
FROM task_run_terminal_commands
WHERE run_id = sqlc.arg(run_id)
  AND task_id = sqlc.arg(task_id)
  AND workspace_id = sqlc.arg(workspace_id);

-- name: ListTaskRunTerminalCommands :many
SELECT
    command_id,
    run_id,
    task_id,
    workspace_id,
    kind,
    phase,
    source_status,
    source_session_id,
    source_claim_token_hash,
    source_lease_until,
    intent_json,
    actor_json,
    command_at,
    admitted_at,
    updated_at
FROM task_run_terminal_commands
ORDER BY admitted_at, run_id;

-- name: AdvanceTaskRunTerminalCommandPhase :execrows
UPDATE task_run_terminal_commands
SET phase = sqlc.arg(next_phase),
    updated_at = sqlc.arg(updated_at)
WHERE command_id = sqlc.arg(command_id)
  AND run_id = sqlc.arg(run_id)
  AND task_id = sqlc.arg(task_id)
  AND workspace_id = sqlc.arg(workspace_id)
  AND phase = sqlc.arg(expected_phase);

-- name: ReleaseTaskRunTerminalCommand :execrows
DELETE FROM task_run_terminal_commands
WHERE task_run_terminal_commands.command_id = sqlc.arg(command_id)
  AND task_run_terminal_commands.run_id = sqlc.arg(run_id)
  AND task_run_terminal_commands.task_id = sqlc.arg(task_id)
  AND task_run_terminal_commands.workspace_id = sqlc.arg(workspace_id)
  AND task_run_terminal_commands.phase = sqlc.arg(expected_phase)
  AND EXISTS (
      SELECT 1
      FROM task_runs
      WHERE task_runs.id = task_run_terminal_commands.run_id
        AND task_runs.task_id = task_run_terminal_commands.task_id
        AND COALESCE(task_runs.workspace_id, '') = task_run_terminal_commands.workspace_id
        AND task_runs.status = task_run_terminal_commands.source_status
        AND COALESCE(task_runs.session_id, '') = task_run_terminal_commands.source_session_id
        AND COALESCE(task_runs.claim_token_hash, '') = task_run_terminal_commands.source_claim_token_hash
        AND COALESCE(task_runs.lease_until, '') = COALESCE(task_run_terminal_commands.source_lease_until, '')
  );

-- name: ConsumeTaskRunTerminalCommand :one
DELETE FROM task_run_terminal_commands
WHERE task_run_terminal_commands.command_id = sqlc.arg(command_id)
  AND task_run_terminal_commands.run_id = sqlc.arg(run_id)
  AND task_run_terminal_commands.task_id = sqlc.arg(task_id)
  AND task_run_terminal_commands.workspace_id = sqlc.arg(workspace_id)
  AND task_run_terminal_commands.phase = sqlc.arg(expected_phase)
  AND EXISTS (
      SELECT 1
      FROM task_runs
      WHERE task_runs.id = task_run_terminal_commands.run_id
        AND task_runs.task_id = task_run_terminal_commands.task_id
        AND COALESCE(task_runs.workspace_id, '') = task_run_terminal_commands.workspace_id
        AND task_runs.status = task_run_terminal_commands.source_status
        AND COALESCE(task_runs.session_id, '') = task_run_terminal_commands.source_session_id
        AND COALESCE(task_runs.claim_token_hash, '') = task_run_terminal_commands.source_claim_token_hash
        AND COALESCE(task_runs.lease_until, '') = COALESCE(task_run_terminal_commands.source_lease_until, '')
  )
RETURNING
    command_id,
    run_id,
    task_id,
    workspace_id,
    kind,
    phase,
    source_status,
    source_session_id,
    source_claim_token_hash,
    source_lease_until,
    intent_json,
    actor_json,
    command_at,
    admitted_at,
    updated_at;
