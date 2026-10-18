-- name: GetNotificationCursor :one
SELECT
  scope_kind,
  profile_id,
  workspace_id,
  consumer_id,
  stream_name,
  subject_id,
  last_sequence,
  last_delivery_id,
  last_delivered_at,
  last_error,
  updated_at
FROM notification_cursors
WHERE scope_kind = sqlc.arg(scope_kind)
  AND profile_id = sqlc.arg(profile_id)
  AND workspace_id = sqlc.arg(workspace_id)
  AND consumer_id = sqlc.arg(consumer_id)
  AND stream_name = sqlc.arg(stream_name)
  AND subject_id = sqlc.arg(subject_id);

-- name: InsertNotificationCursor :exec
INSERT INTO notification_cursors (
  profile_id,
  scope_kind,
  workspace_id,
  consumer_id,
  stream_name,
  subject_id,
  last_sequence,
  last_delivery_id,
  last_delivered_at,
  last_error,
  updated_at
) VALUES (
  sqlc.arg(profile_id),
  sqlc.arg(scope_kind),
  sqlc.arg(workspace_id),
  sqlc.arg(consumer_id),
  sqlc.arg(stream_name),
  sqlc.arg(subject_id),
  sqlc.arg(last_sequence),
  sqlc.arg(last_delivery_id),
  sqlc.narg(last_delivered_at),
  '',
  sqlc.arg(updated_at)
);

-- name: UpdateNotificationCursor :exec
UPDATE notification_cursors
SET last_sequence = sqlc.arg(last_sequence),
    last_delivery_id = sqlc.arg(last_delivery_id),
    last_delivered_at = sqlc.narg(last_delivered_at),
    last_error = '',
    updated_at = sqlc.arg(updated_at)
WHERE consumer_id = sqlc.arg(consumer_id)
  AND profile_id = sqlc.arg(profile_id)
  AND scope_kind = sqlc.arg(scope_kind)
  AND workspace_id = sqlc.arg(workspace_id)
  AND stream_name = sqlc.arg(stream_name)
  AND subject_id = sqlc.arg(subject_id);

-- name: ResetNotificationCursor :exec
INSERT INTO notification_cursors (
  profile_id,
  scope_kind,
  workspace_id,
  consumer_id,
  stream_name,
  subject_id,
  last_sequence,
  last_delivery_id,
  last_delivered_at,
  last_error,
  updated_at
) VALUES (
  sqlc.arg(profile_id),
  sqlc.arg(scope_kind),
  sqlc.arg(workspace_id),
  sqlc.arg(consumer_id),
  sqlc.arg(stream_name),
  sqlc.arg(subject_id),
  sqlc.arg(last_sequence),
  sqlc.arg(last_delivery_id),
  sqlc.narg(last_delivered_at),
  '',
  sqlc.arg(updated_at)
)
ON CONFLICT(scope_kind, profile_id, workspace_id, consumer_id, stream_name, subject_id) DO UPDATE SET
  last_sequence = excluded.last_sequence,
  last_delivery_id = excluded.last_delivery_id,
  last_delivered_at = excluded.last_delivered_at,
  last_error = '',
  updated_at = excluded.updated_at;

-- name: RecordNotificationCursorError :exec
INSERT INTO notification_cursors (
  profile_id,
  scope_kind,
  workspace_id,
  consumer_id,
  stream_name,
  subject_id,
  last_sequence,
  last_delivery_id,
  last_delivered_at,
  last_error,
  updated_at
) VALUES (
  sqlc.arg(profile_id),
  sqlc.arg(scope_kind),
  sqlc.arg(workspace_id),
  sqlc.arg(consumer_id),
  sqlc.arg(stream_name),
  sqlc.arg(subject_id),
  0,
  '',
  NULL,
  sqlc.arg(last_error),
  sqlc.arg(updated_at)
)
ON CONFLICT(scope_kind, profile_id, workspace_id, consumer_id, stream_name, subject_id) DO UPDATE SET
  last_error = excluded.last_error,
  updated_at = excluded.updated_at;
