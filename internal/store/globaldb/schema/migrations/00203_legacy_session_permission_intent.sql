-- +goose Up
-- v0.7.0 enforced session lists only for spawned/coordinator sessions.
-- The 00202 application time excludes sessions authored with the new intent markers.
UPDATE sessions
SET permission_policy_json = json_set(
    permission_policy_json,
    '$.mcp_servers_mode', 'inherit',
    '$.tools_mode', CASE WHEN EXISTS (
        SELECT 1 FROM session_creation_profiles
        WHERE profile_ref = sessions.creation_profile_ref
          AND COALESCE(json_array_length(profile_json, '$.allowed_tools'), 0) > 0
    ) THEN 'list' ELSE 'inherit' END
)
WHERE lineage_kind <> 'spawn'
  AND session_type NOT IN ('spawned', 'coordinator')
  AND json_type(permission_policy_json, '$.ancestor_mcp_ceiling') IS NULL
  AND julianday(created_at) <= (
      SELECT julianday(tstamp) FROM goose_db_version_global
      WHERE version_id = 202 AND is_applied = 1
      ORDER BY id DESC LIMIT 1
  )
  AND (json_extract(permission_policy_json, '$.tools_mode') = 'list'
       OR json_extract(permission_policy_json, '$.mcp_servers_mode') = 'list');

-- +goose Down
-- Restoring the lossy backfill would revoke access; retain the corrected intent.
