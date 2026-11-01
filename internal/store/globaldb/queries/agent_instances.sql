-- name: GetAgentInstance :one
SELECT * FROM agent_instances WHERE id = ?;

-- name: FindLiveAgentInstance :one
SELECT * FROM agent_instances WHERE profile_id = ? AND agent_name = ? AND workspace_id = ? AND archived_at IS NULL;

-- name: CountActiveAgentInstances :one
SELECT count(*) FROM agent_instances WHERE profile_id = ? AND archived_at IS NULL AND state IN ('starting', 'active');

-- name: InsertAgentInstance :exec
INSERT INTO agent_instances (id, profile_id, agent_name, workspace_id, state, grant_ceiling, permission_ceiling, depth, created_by_kind, created_by_id, created_at, updated_at)
VALUES (?, ?, ?, ?, 'starting', ?, ?, ?, ?, ?, ?, ?);

-- name: BindAgentInstanceMain :execrows
UPDATE agent_instances SET main_session_id = sqlc.arg(session_id), state = 'active', updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND state = 'starting' AND archived_at IS NULL AND main_session_id IS NULL;

-- name: PauseAgentInstance :execrows
UPDATE agent_instances SET state = 'paused', pause_reason = ?, updated_at = ?
WHERE id = ? AND state = 'active' AND archived_at IS NULL;

-- name: ResumeAgentInstance :execrows
UPDATE agent_instances SET state = 'active', pause_reason = '', updated_at = ?
WHERE id = ? AND state = 'paused' AND archived_at IS NULL;

-- name: ArchiveAgentInstance :execrows
UPDATE agent_instances SET archived_at = ?, updated_at = ? WHERE id = ? AND archived_at IS NULL;

-- name: ArchiveWorkspaceAgentInstances :exec
UPDATE agent_instances SET archived_at = sqlc.arg(archived_at), updated_at = sqlc.arg(updated_at)
WHERE workspace_id = sqlc.arg(workspace_id) AND archived_at IS NULL;

-- name: DeleteUnstartedAgentInstance :execrows
DELETE FROM agent_instances WHERE agent_instances.id = ? AND state = 'starting' AND main_session_id IS NULL
AND NOT EXISTS (SELECT 1 FROM sessions WHERE instance_id = agent_instances.id);

-- name: ListAgentInstances :many
SELECT * FROM agent_instances WHERE profile_id = sqlc.arg(profile_id)
AND (sqlc.arg(all_workspaces) = 1 OR workspace_id = sqlc.arg(workspace_id))
AND archived_at IS NULL AND id > sqlc.arg(after_id) ORDER BY id LIMIT sqlc.arg(page_limit);

-- name: IsInstanceMainSession :one
SELECT EXISTS(SELECT 1 FROM agent_instances WHERE main_session_id = ? AND archived_at IS NULL);

-- name: StampInstanceMainSession :execrows
UPDATE sessions SET instance_id = sqlc.arg(instance_id)
WHERE sessions.id = sqlc.arg(session_id) AND archived_at IS NULL
AND (parent_session_id IS NULL OR parent_session_id = '')
AND (instance_id IS NULL OR instance_id = sqlc.arg(instance_id))
AND EXISTS (SELECT 1 FROM agent_instances i WHERE i.id = sqlc.arg(instance_id)
AND i.agent_name = sessions.agent_name AND i.profile_id = sessions.profile_id
AND i.workspace_id = sessions.workspace_id AND i.archived_at IS NULL);

-- name: PublishMissingInstanceMain :execrows
UPDATE agent_instances SET main_session_id=sqlc.arg(session_id), updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND main_session_id IS NULL AND archived_at IS NULL;

-- name: ListInstancesForReconciliation :many
SELECT * FROM agent_instances WHERE archived_at IS NULL AND id > sqlc.arg(after_id)
ORDER BY id LIMIT sqlc.arg(page_limit);

-- name: GetInstanceCaller :one
WITH RECURSIVE ancestry(id, parent_session_id, instance_id) AS (
SELECT id, parent_session_id, instance_id FROM sessions WHERE id=sqlc.arg(session_id)
UNION
SELECT s.id, s.parent_session_id, s.instance_id FROM sessions s JOIN ancestry a ON s.id=a.parent_session_id
)
SELECT s.id, s.agent_name, s.profile_id, s.permission_policy_json, s.effective_workspace_access,
COALESCE((SELECT i.id FROM ancestry a JOIN agent_instances i ON i.id=a.instance_id LIMIT 1),'') AS originating_instance_id,
COALESCE((SELECT MAX(i.depth) FROM ancestry a JOIN agent_instances i ON i.id=a.instance_id),0) AS instance_depth
FROM sessions s WHERE s.id=sqlc.arg(session_id);

-- name: GetInstanceMainAttempt :one
SELECT id FROM sessions WHERE instance_id = ? AND archived_at IS NULL
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: DetachFailedInstanceAttempts :exec
UPDATE sessions SET instance_id = NULL
WHERE instance_id = sqlc.arg(instance_id) AND archived_at IS NOT NULL
AND EXISTS (SELECT 1 FROM agent_instances i WHERE i.id = sqlc.arg(instance_id)
AND i.state = 'starting' AND i.main_session_id IS NULL);

-- name: IsSupersededInstanceSession :one
SELECT EXISTS (
 SELECT 1 FROM sessions s JOIN agent_instances i ON i.id=s.instance_id
 WHERE s.id=sqlc.arg(session_id) AND i.main_session_id IS NOT NULL
 AND i.main_session_id<>s.id
);
