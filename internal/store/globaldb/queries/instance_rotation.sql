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
AND NOT EXISTS(SELECT 1 FROM session_input_queue WHERE session_id=sqlc.arg(session_id) AND status IN ('queued','dispatching'));

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
