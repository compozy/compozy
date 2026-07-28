-- name: UpsertToolProcessRecord :exec
INSERT INTO tool_processes (
  id, source, session_id, turn_id, tool_call_id, terminal_id, extension_name,
  hook_name, sandbox_id, pid, process_group_id, command, args_json, cwd,
  started_at, started_by_pid, state, exit_code, error, created_at, updated_at, completed_at
) VALUES (
  sqlc.arg(id), sqlc.arg(source), sqlc.arg(session_id), sqlc.arg(turn_id),
  sqlc.arg(tool_call_id), sqlc.arg(terminal_id), sqlc.arg(extension_name),
  sqlc.arg(hook_name), sqlc.arg(sandbox_id), sqlc.arg(pid), sqlc.arg(process_group_id),
  sqlc.arg(command), sqlc.arg(args_json), sqlc.arg(cwd), sqlc.narg(started_at),
  sqlc.arg(started_by_pid), sqlc.arg(state), sqlc.narg(exit_code), sqlc.arg(error),
  sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.narg(completed_at)
)
ON CONFLICT(id) DO UPDATE SET
  source = excluded.source, session_id = excluded.session_id, turn_id = excluded.turn_id,
  tool_call_id = excluded.tool_call_id, terminal_id = excluded.terminal_id,
  extension_name = excluded.extension_name, hook_name = excluded.hook_name,
  sandbox_id = excluded.sandbox_id, pid = excluded.pid,
  process_group_id = excluded.process_group_id, command = excluded.command,
  args_json = excluded.args_json, cwd = excluded.cwd, started_at = excluded.started_at,
  started_by_pid = excluded.started_by_pid, state = excluded.state,
  exit_code = excluded.exit_code, error = excluded.error, updated_at = excluded.updated_at,
  completed_at = excluded.completed_at;

-- name: UpdateToolProcessRecordState :execrows
UPDATE tool_processes
SET state = sqlc.arg(state), exit_code = sqlc.narg(exit_code), error = sqlc.arg(error),
    updated_at = sqlc.arg(updated_at), completed_at = sqlc.narg(completed_at)
WHERE id = sqlc.arg(id);
