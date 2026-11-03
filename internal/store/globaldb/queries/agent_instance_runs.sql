-- name: ListInstanceManagedRuns :many
WITH RECURSIVE instance_sessions(id, profile_id) AS (
 SELECT s.id, s.profile_id FROM sessions s JOIN agent_instances i ON s.instance_id = i.id
 WHERE i.id = sqlc.arg(instance_id) AND s.profile_id = i.profile_id
 UNION
 SELECT s.id, s.profile_id FROM sessions s JOIN instance_sessions parent ON s.parent_session_id = parent.id
 WHERE s.profile_id = parent.profile_id
), owned_runs(id) AS (
 SELECT r.id FROM loop_runs r WHERE r.historical = 0 AND r.status = 'running'
 AND EXISTS (
  SELECT 1 FROM instance_sessions s WHERE s.profile_id = r.profile_id AND (
   r.origin_session_id = s.id OR (r.started_by_kind = 'agent_session' AND r.started_by_ref = s.id)
   OR EXISTS (SELECT 1 FROM loop_session_bindings b WHERE b.loop_run_id = r.id AND b.session_id = s.id AND b.state IN ('active','creating'))
   OR EXISTS (SELECT 1 FROM task_runs t WHERE t.loop_run_id = r.id AND t.session_id = s.id AND t.status IN ('claimed','starting','running'))
  )
 )
 UNION
 SELECT run_id FROM agent_instance_paused_runs WHERE instance_id = sqlc.arg(instance_id)
)
SELECT r.id, r.workspace_id, r.status, r.pause_requested,
 EXISTS (SELECT 1 FROM agent_instance_paused_runs p WHERE p.instance_id = sqlc.arg(instance_id) AND p.run_id = r.id) AS recorded
FROM loop_runs r JOIN owned_runs owned ON owned.id = r.id
WHERE r.id > sqlc.arg(after_id) ORDER BY r.id LIMIT sqlc.arg(page_limit);

-- name: GetPausedInstanceForLoopRun :one
WITH RECURSIVE lineage(id, parent_session_id, instance_id, profile_id) AS (
 SELECT s.id, s.parent_session_id, s.instance_id, s.profile_id FROM sessions s JOIN loop_runs r ON s.profile_id = r.profile_id
 WHERE r.id = sqlc.arg(run_id) AND (
  r.origin_session_id = s.id OR (r.started_by_kind = 'agent_session' AND r.started_by_ref = s.id)
  OR EXISTS (SELECT 1 FROM loop_session_bindings b WHERE b.loop_run_id = r.id AND b.session_id = s.id AND b.state IN ('active','creating'))
  OR EXISTS (SELECT 1 FROM task_runs t WHERE t.loop_run_id = r.id AND t.session_id = s.id AND t.status IN ('claimed','starting','running'))
 )
 UNION
 SELECT s.id, s.parent_session_id, s.instance_id, s.profile_id FROM sessions s JOIN lineage child ON s.id = child.parent_session_id
 WHERE s.profile_id = child.profile_id
)
SELECT i.id FROM agent_instances i JOIN lineage s ON s.instance_id = i.id AND s.profile_id = i.profile_id
WHERE i.state = 'paused' AND i.archived_at IS NULL LIMIT 1;

-- name: RecordInstancePausedRun :exec
INSERT INTO agent_instance_paused_runs(instance_id, run_kind, run_id, paused_at)
SELECT sqlc.arg(instance_id), CASE WHEN r.origin_kind = 'session' THEN 'goal' ELSE 'loop' END, r.id, sqlc.arg(paused_at)
FROM loop_runs r WHERE r.id = sqlc.arg(run_id)
ON CONFLICT(instance_id, run_kind, run_id) DO NOTHING;

-- name: DeleteInstancePausedRun :exec
DELETE FROM agent_instance_paused_runs WHERE run_id = sqlc.arg(run_id);

-- name: ForgetSettledInstanceRuns :exec
DELETE FROM agent_instance_paused_runs
WHERE instance_id = sqlc.arg(instance_id) AND NOT EXISTS (
 SELECT 1 FROM loop_runs r WHERE r.id = agent_instance_paused_runs.run_id
 AND (r.status = 'paused' OR (r.status = 'running' AND r.pause_requested = 1))
);
