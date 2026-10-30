-- name: ListSessionProcessObservations :many
SELECT * FROM session_process_observations ORDER BY pid, started_at;

-- name: UpsertSessionProcessObservation :exec
INSERT INTO session_process_observations (
  pid, started_at, session_id, workspace_id, profile_id, session_name,
  pgid, first_seen, group_observed_at, attribution
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(pid, started_at) DO UPDATE SET
  session_name = excluded.session_name,
  pgid = excluded.pgid,
  group_observed_at = excluded.group_observed_at,
  attribution = excluded.attribution;

-- name: DeleteSessionProcessObservation :exec
DELETE FROM session_process_observations WHERE pid = ? AND started_at = ?;
