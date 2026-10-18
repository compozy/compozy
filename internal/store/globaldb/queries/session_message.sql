-- name: SessionMessageRateWindow :one
SELECT
  COUNT(*) AS message_count,
  CAST(COALESCE(MIN(created_at), '') AS TEXT) AS oldest_created_at
FROM session_prompt_admissions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND session_id = sqlc.arg(session_id)
  AND created_at >= sqlc.arg(since)
  AND origin_json IS NOT NULL
  AND json_extract(origin_json, '$.session_id') = CAST(sqlc.arg(sender_session_id) AS TEXT);
