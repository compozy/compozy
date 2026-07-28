-- name: GetAutomationSuggestion :one
SELECT id, workspace_id, source, dedup_key, status, payload, created_at, resolved_at
FROM automation_suggestions
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id);

-- name: GetAutomationSuggestionByDedupKey :one
SELECT id, workspace_id, source, dedup_key, status, payload, created_at, resolved_at
FROM automation_suggestions
WHERE workspace_id = sqlc.arg(workspace_id) AND dedup_key = sqlc.arg(dedup_key);

-- name: CountPendingAutomationSuggestions :one
SELECT COUNT(*) FROM automation_suggestions
WHERE workspace_id = sqlc.arg(workspace_id) AND status = 'pending';

-- name: InsertAutomationSuggestion :exec
INSERT INTO automation_suggestions (
  id, workspace_id, source, dedup_key, status, payload, created_at, resolved_at
) VALUES (
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(source), sqlc.arg(dedup_key),
  sqlc.arg(status), sqlc.arg(payload), sqlc.arg(created_at), sqlc.narg(resolved_at)
);

-- name: ListAutomationSuggestions :many
SELECT id, workspace_id, source, dedup_key, status, payload, created_at, resolved_at
FROM automation_suggestions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND (CAST(sqlc.arg(status) AS TEXT) = '' OR status = CAST(sqlc.arg(status) AS TEXT))
ORDER BY created_at ASC, id ASC;

-- name: ListAcceptedAutomationSuggestions :many
SELECT id, workspace_id, source, dedup_key, status, payload, created_at, resolved_at
FROM automation_suggestions
WHERE status = 'accepted'
ORDER BY created_at ASC, id ASC;

-- name: ResolveAutomationSuggestion :execrows
UPDATE automation_suggestions
SET status = sqlc.arg(status), resolved_at = sqlc.arg(resolved_at)
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id) AND status = 'pending';

-- name: RollbackAutomationSuggestionAcceptance :execrows
UPDATE automation_suggestions
SET status = 'pending', resolved_at = NULL
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id)
  AND status = 'accepted' AND resolved_at = sqlc.arg(resolved_at);

-- name: InsertAutomationSuggestionEventSummary :exec
INSERT INTO event_summaries (
  id, workspace_id, type, content_json, actor_kind, actor_id, outcome, summary, timestamp
) VALUES (
  sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(type), sqlc.arg(content_json),
  'automation_suggestion', sqlc.arg(actor_id), sqlc.arg(outcome), sqlc.arg(summary),
  sqlc.arg(timestamp)
)
ON CONFLICT(id) DO NOTHING;
