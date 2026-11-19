-- name: GetTokenUsageReceiptProfile :one
SELECT profile_id FROM token_usage_receipts
WHERE session_id = sqlc.arg(session_id) AND turn_id = sqlc.arg(turn_id);

-- name: GetTokenUsageReceiptSessionOwner :one
WITH RECURSIVE ancestry(id, parent_session_id, instance_id, profile_id, lineage_kind, nearest_instance_id) AS (
    SELECT id, parent_session_id, instance_id, profile_id, lineage_kind,
        COALESCE(NULLIF(instance_id, ''), '')
    FROM sessions WHERE id = sqlc.arg(session_id)
    UNION
    SELECT s.id, s.parent_session_id, s.instance_id, s.profile_id, s.lineage_kind,
        CASE WHEN a.nearest_instance_id != '' THEN a.nearest_instance_id
             ELSE COALESCE(NULLIF(s.instance_id, ''), '') END
    FROM sessions s JOIN ancestry a ON s.id = a.parent_session_id
    WHERE a.lineage_kind NOT IN ('continue', 'fork')
      AND a.parent_session_id IS NOT NULL AND a.parent_session_id != ''
)
SELECT owner.profile_id,
    CAST(COALESCE((
        SELECT a.nearest_instance_id FROM ancestry a
        WHERE a.lineage_kind IN ('continue', 'fork')
           OR a.parent_session_id IS NULL OR a.parent_session_id = ''
        LIMIT 1
    ), '') AS TEXT) AS instance_id
FROM sessions owner
WHERE owner.id = sqlc.arg(session_id)
  AND EXISTS (SELECT 1 FROM ancestry a
      WHERE a.lineage_kind IN ('continue', 'fork')
         OR a.parent_session_id IS NULL OR a.parent_session_id = '')
  AND NOT EXISTS (
      SELECT 1 FROM ancestry a WHERE a.profile_id != owner.profile_id
      OR (a.lineage_kind NOT IN ('continue', 'fork')
          AND a.parent_session_id IS NOT NULL AND a.parent_session_id != ''
          AND NOT EXISTS (SELECT 1 FROM sessions parent WHERE parent.id = a.parent_session_id))
      OR (a.instance_id IS NOT NULL AND a.instance_id != ''
          AND NOT EXISTS (SELECT 1 FROM agent_instances i
              WHERE i.id = a.instance_id AND i.profile_id = owner.profile_id))
  );

-- name: InsertTokenUsageReceipt :execrows
INSERT INTO token_usage_receipts (
    profile_id, session_id, turn_id, instance_id, day,
    total_tokens, usd_amount, cost_status, created_at
) VALUES (
    sqlc.arg(profile_id), sqlc.arg(session_id), sqlc.arg(turn_id),
    sqlc.arg(instance_id), sqlc.arg(day), sqlc.arg(total_tokens),
    sqlc.narg(usd_amount), sqlc.arg(cost_status), sqlc.arg(created_at)
) ON CONFLICT(session_id, turn_id) DO NOTHING;
