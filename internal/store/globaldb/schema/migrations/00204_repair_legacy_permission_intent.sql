-- +goose Up
-- v0.7.0 enforced each depth-above-zero instance descendant's own stored lists.
WITH RECURSIVE delegated_lineage(id, profile_id) AS (
    SELECT s.id, s.profile_id FROM sessions s
    JOIN agent_instances i ON i.id = s.instance_id AND i.profile_id = s.profile_id
    WHERE i.depth > 0
    UNION
    SELECT s.id, s.profile_id FROM sessions s JOIN delegated_lineage p
      ON s.parent_session_id = p.id AND s.profile_id = p.profile_id
)
UPDATE sessions
SET permission_policy_json = json_set(
    permission_policy_json, '$.mcp_servers_mode', 'list', '$.tools_mode', 'list'
)
WHERE id IN (SELECT id FROM delegated_lineage)
  AND json_valid(permission_policy_json)
  AND json_type(permission_policy_json, '$.ancestor_mcp_ceiling') IS NULL
  AND julianday(created_at) <= (
      SELECT julianday(tstamp) FROM goose_db_version_global
      WHERE version_id = 202 AND is_applied = 1
      ORDER BY id DESC LIMIT 1
  );

-- +goose Down
-- Intent corrections are lossless and must not be reversed into wider authority.
