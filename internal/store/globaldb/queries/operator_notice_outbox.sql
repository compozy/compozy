-- name: InsertOperatorNotice :exec
INSERT INTO operator_notice_outbox (
    id, profile_id, workspace_id, session_id, kind, action, resource, title, body, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListUnreadOperatorNotices :many
SELECT n.* FROM operator_notice_outbox AS n
WHERE n.profile_id = sqlc.arg(profile_id)
  AND NOT EXISTS (
    SELECT 1 FROM attention_acknowledgements AS receipt
    WHERE receipt.profile_id = n.profile_id
      AND receipt.actor_kind = sqlc.arg(actor_kind)
      AND receipt.actor_id = sqlc.arg(actor_id)
      AND receipt.occurrence_id = n.id
  )
ORDER BY n.created_at DESC, n.id;
