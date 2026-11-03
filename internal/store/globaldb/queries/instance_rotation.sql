-- name: InstanceRotationAdmissionCount :one
SELECT count(*) FROM session_prompt_admissions WHERE session_id=sqlc.arg(session_id);

-- name: InstanceRotationPersistenceReady :one
WITH RECURSIVE descendants(id) AS (
 SELECT id FROM sessions WHERE parent_session_id=sqlc.arg(session_id)
 UNION SELECT s.id FROM sessions s JOIN descendants d ON s.parent_session_id=d.id
)
SELECT EXISTS(SELECT 1 FROM sessions s WHERE s.id=sqlc.arg(session_id)
 AND s.state='active' AND trim(COALESCE(json_extract(CASE WHEN json_valid(s.activity_json) THEN s.activity_json ELSE '{}' END, '$.turn_id'), ''))='' AND s.archived_at IS NULL)
AND NOT EXISTS(SELECT 1 FROM sessions s JOIN descendants d ON s.id=d.id WHERE s.state NOT IN ('stopped','failed','crashed'))
AND NOT EXISTS(SELECT 1 FROM session_subagents WHERE parent_session_id=sqlc.arg(session_id)
 AND origin='delegated' AND delivery NOT IN ('delivered','acknowledged','disposed'))
AND NOT EXISTS(SELECT 1 FROM session_subagent_wakes WHERE parent_session_id=sqlc.arg(session_id) AND state IN ('open','dispatched'))
AND NOT EXISTS(SELECT 1 FROM session_pending_interactions WHERE session_id=sqlc.arg(session_id) AND status IN ('pending','orphaned'))
AND NOT EXISTS(SELECT 1 FROM session_prompt_reply_watches WHERE (sender_session_id=sqlc.arg(session_id) OR target_session_id=sqlc.arg(session_id)) AND state IN ('armed','fired'))
AND NOT EXISTS(SELECT 1 FROM session_prompt_admissions WHERE session_id=sqlc.arg(session_id) AND state IN ('reserved','dispatch_committed'))
AND NOT EXISTS(SELECT 1 FROM session_input_queue WHERE session_id=sqlc.arg(session_id) AND status IN ('queued','dispatching'))
AND NOT EXISTS(SELECT 1 FROM loop_runs WHERE origin_kind='session' AND origin_session_id=sqlc.arg(session_id)
 AND historical=0 AND status IN ('queued','running','watching','needs-approval','paused'));

-- name: PublishRotatedInstanceMain :execrows
UPDATE agent_instances SET main_session_id=sqlc.arg(successor_id), updated_at=sqlc.arg(updated_at)
WHERE agent_instances.id=sqlc.arg(instance_id) AND main_session_id=sqlc.arg(predecessor_id)
AND state='active' AND archived_at IS NULL
AND EXISTS(SELECT 1 FROM sessions s WHERE s.id=sqlc.arg(successor_id)
 AND s.instance_id=agent_instances.id AND s.state='active' AND trim(COALESCE(json_extract(CASE WHEN json_valid(s.activity_json) THEN s.activity_json ELSE '{}' END, '$.turn_id'), ''))='' AND s.archived_at IS NULL);

-- name: CanRegisterRotationCandidate :one
SELECT EXISTS(SELECT 1 FROM agent_instances i WHERE i.id=sqlc.arg(instance_id)
 AND i.state='active' AND i.archived_at IS NULL AND i.main_session_id IS NOT NULL
 AND NOT EXISTS(SELECT 1 FROM sessions s WHERE s.instance_id=i.id
 AND s.id<>i.main_session_id AND s.state<>'stopped' AND s.archived_at IS NULL));

-- name: ListSupersededInstanceSessions :many
SELECT s.id FROM sessions s JOIN agent_instances i ON i.id=s.instance_id
WHERE i.id=sqlc.arg(instance_id) AND i.main_session_id IS NOT NULL
AND s.id<>i.main_session_id AND s.state<>'stopped' AND s.archived_at IS NULL
ORDER BY s.created_at, s.id;

-- name: BeginInstanceRotationWait :one
INSERT INTO agent_instance_rotation_wait(instance_id,main_session_id,started_at)
SELECT sqlc.arg(instance_id),sqlc.arg(main_session_id),sqlc.arg(started_at)
WHERE EXISTS(SELECT 1 FROM agent_instances i WHERE i.id=sqlc.arg(instance_id)
AND i.main_session_id=sqlc.arg(main_session_id) AND i.state='active' AND i.archived_at IS NULL)
ON CONFLICT(instance_id) DO UPDATE SET
started_at=CASE WHEN agent_instance_rotation_wait.main_session_id=excluded.main_session_id THEN agent_instance_rotation_wait.started_at ELSE excluded.started_at END,
main_session_id=excluded.main_session_id
RETURNING started_at;

-- name: ClearInstanceRotationWait :exec
DELETE FROM agent_instance_rotation_wait WHERE instance_id=sqlc.arg(instance_id) AND main_session_id=sqlc.arg(main_session_id);

-- name: MarkInstanceRotationDue :exec
UPDATE agent_instance_wake_state SET rotation_due_session=sqlc.arg(session_id)
WHERE instance_id=sqlc.arg(instance_id) AND EXISTS(SELECT 1 FROM agent_instances
 WHERE id=sqlc.arg(instance_id) AND main_session_id=sqlc.arg(session_id) AND archived_at IS NULL);

-- name: ClearInstanceRotationDue :exec
UPDATE agent_instance_wake_state SET rotation_due_session=''
WHERE instance_id=sqlc.arg(instance_id) AND rotation_due_session=sqlc.arg(session_id);

-- name: InstanceRotationBlockers :many
WITH RECURSIVE descendants(id,state) AS (
 SELECT root.id,root.state FROM sessions root WHERE root.parent_session_id=sqlc.arg(session_id)
 UNION SELECT s.id,s.state FROM sessions s JOIN descendants d ON s.parent_session_id=d.id
)
SELECT 'queued_input' AS reason WHERE EXISTS(SELECT 1 FROM session_input_queue WHERE session_id=sqlc.arg(session_id) AND status IN ('queued','dispatching'))
UNION ALL SELECT 'active_goal' WHERE EXISTS(SELECT 1 FROM loop_runs WHERE origin_kind='session' AND origin_session_id=sqlc.arg(session_id) AND historical=0 AND status IN ('queued','running','watching','needs-approval','paused'))
UNION ALL SELECT 'prompt_admission' WHERE EXISTS(SELECT 1 FROM session_prompt_admissions WHERE session_id=sqlc.arg(session_id) AND state IN ('reserved','dispatch_committed'))
UNION ALL SELECT 'pending_interaction' WHERE EXISTS(SELECT 1 FROM session_pending_interactions WHERE session_id=sqlc.arg(session_id) AND status IN ('pending','orphaned'))
UNION ALL SELECT 'reply_watch' WHERE EXISTS(SELECT 1 FROM session_prompt_reply_watches WHERE (sender_session_id=sqlc.arg(session_id) OR target_session_id=sqlc.arg(session_id)) AND state IN ('armed','fired'))
UNION ALL SELECT 'child_wake' WHERE EXISTS(SELECT 1 FROM session_subagent_wakes WHERE parent_session_id=sqlc.arg(session_id) AND state IN ('open','dispatched'))
UNION ALL SELECT 'child_delivery' WHERE EXISTS(SELECT 1 FROM session_subagents WHERE parent_session_id=sqlc.arg(session_id) AND origin='delegated' AND delivery NOT IN ('delivered','acknowledged','disposed'))
UNION ALL SELECT 'live_descendant' WHERE EXISTS(SELECT 1 FROM descendants WHERE state NOT IN ('stopped','failed','crashed'));
