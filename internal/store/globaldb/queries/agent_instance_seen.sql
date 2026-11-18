-- name: GetAgentInstanceSeen :one
SELECT last_seen_at FROM agent_instance_seen
WHERE instance_id = ? AND operator_id = ?;

-- name: MarkAgentInstanceSeen :one
INSERT INTO agent_instance_seen (instance_id, operator_id, last_seen_at)
VALUES (?, ?, ?)
ON CONFLICT(instance_id, operator_id) DO UPDATE SET
last_seen_at = MAX(agent_instance_seen.last_seen_at, excluded.last_seen_at)
RETURNING last_seen_at;

-- name: ListSeenStoppedInstanceInteractions :many
WITH RECURSIVE instance_sessions(id, profile_id) AS (
 SELECT s.id, s.profile_id FROM sessions s JOIN agent_instances i ON s.instance_id = i.id
 WHERE i.id = sqlc.arg(instance_id) AND s.profile_id = i.profile_id
 UNION
 SELECT s.id, s.profile_id FROM sessions s JOIN instance_sessions parent ON s.parent_session_id = parent.id
 WHERE s.profile_id = parent.profile_id
)
SELECT p.interaction_id FROM session_pending_interactions p
JOIN instance_sessions tree ON tree.id = p.session_id JOIN sessions s ON s.id = tree.id
WHERE s.state = 'stopped' AND p.status IN ('pending', 'orphaned') AND p.created_at <= sqlc.arg(seen_at);
