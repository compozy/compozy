-- name: GetAgentInstanceSeen :one
SELECT last_seen_at FROM agent_instance_seen
WHERE instance_id = ? AND operator_id = ?;

-- name: MarkAgentInstanceSeen :one
INSERT INTO agent_instance_seen (instance_id, operator_id, last_seen_at)
VALUES (?, ?, ?)
ON CONFLICT(instance_id, operator_id) DO UPDATE SET
last_seen_at = MAX(agent_instance_seen.last_seen_at, excluded.last_seen_at)
RETURNING last_seen_at;
