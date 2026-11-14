-- +goose Up
UPDATE sessions
SET permission_policy_json = json_set(
    permission_policy_json,
    '$.mcp_servers_mode', COALESCE(NULLIF(json_extract(permission_policy_json, '$.mcp_servers_mode'), ''),
        CASE WHEN lineage_kind = 'spawn' OR session_type IN ('spawned', 'coordinator') THEN 'list'
             WHEN COALESCE(json_array_length(permission_policy_json, '$.mcp_servers'), 0) > 0
                  AND COALESCE(json_extract(permission_policy_json, '$.materialized_mcp_servers'), 0) = 0 THEN 'list'
             ELSE 'inherit' END),
    '$.tools_mode', COALESCE(NULLIF(json_extract(permission_policy_json, '$.tools_mode'), ''),
        CASE WHEN lineage_kind = 'spawn' OR session_type IN ('spawned', 'coordinator') THEN 'list'
             WHEN json_extract(permission_policy_json, '$.materialized_tools') = 1 THEN 'inherit'
             ELSE 'list' END)
)
WHERE COALESCE(json_extract(permission_policy_json, '$.mcp_servers_mode'), '') = ''
   OR COALESCE(json_extract(permission_policy_json, '$.tools_mode'), '') = '';

UPDATE agent_instances
SET permission_ceiling = json_set(
    permission_ceiling,
    '$.mcp_servers_mode', COALESCE(NULLIF(json_extract(permission_ceiling, '$.mcp_servers_mode'), ''),
        CASE WHEN depth = 0 THEN 'inherit' ELSE 'list' END),
    '$.tools_mode', COALESCE(NULLIF(json_extract(permission_ceiling, '$.tools_mode'), ''),
        CASE WHEN depth = 0 THEN 'inherit' ELSE 'list' END)
)
WHERE COALESCE(json_extract(permission_ceiling, '$.mcp_servers_mode'), '') = ''
   OR COALESCE(json_extract(permission_ceiling, '$.tools_mode'), '') = '';
