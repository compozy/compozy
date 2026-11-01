-- name: ListSessionMonitors :many
SELECT * FROM session_monitors ORDER BY created_at, id;

-- name: UpsertSessionMonitor :exec
INSERT INTO session_monitors (
  id, session_id, workspace_id, profile_id, kind, description, terminal_id, until, pattern, timeout_ms,
  status, created_at, deadline_at, fired_at, reason, exit_code, owner_binding, owner_check_failed_at, version
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  status = excluded.status,
  fired_at = excluded.fired_at,
  reason = excluded.reason,
  exit_code = excluded.exit_code,
  owner_binding = excluded.owner_binding,
  owner_check_failed_at = excluded.owner_check_failed_at,
  version = excluded.version
WHERE excluded.version > session_monitors.version;

-- name: PruneFinishedSessionMonitors :exec
DELETE FROM session_monitors
WHERE status NOT IN ('active', 'delivering') AND fired_at != '' AND fired_at < ?;
