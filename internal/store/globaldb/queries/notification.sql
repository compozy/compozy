-- name: GetNotificationCursor :one
SELECT
  consumer_id,
  stream_name,
  subject_id,
  last_sequence,
  last_delivery_id,
  last_delivered_at,
  last_error,
  updated_at
FROM notification_cursors
WHERE consumer_id = sqlc.arg(consumer_id)
  AND stream_name = sqlc.arg(stream_name)
  AND subject_id = sqlc.arg(subject_id);

-- name: InsertNotificationCursor :exec
INSERT INTO notification_cursors (
  consumer_id,
  stream_name,
  subject_id,
  last_sequence,
  last_delivery_id,
  last_delivered_at,
  last_error,
  updated_at
) VALUES (
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
  AND stream_name = sqlc.arg(stream_name)
  AND subject_id = sqlc.arg(subject_id);

-- name: ResetNotificationCursor :exec
INSERT INTO notification_cursors (
  consumer_id,
  stream_name,
  subject_id,
  last_sequence,
  last_delivery_id,
  last_delivered_at,
  last_error,
  updated_at
) VALUES (
  sqlc.arg(consumer_id),
  sqlc.arg(stream_name),
  sqlc.arg(subject_id),
  sqlc.arg(last_sequence),
  sqlc.arg(last_delivery_id),
  sqlc.narg(last_delivered_at),
  '',
  sqlc.arg(updated_at)
)
ON CONFLICT(consumer_id, stream_name, subject_id) DO UPDATE SET
  last_sequence = excluded.last_sequence,
  last_delivery_id = excluded.last_delivery_id,
  last_delivered_at = excluded.last_delivered_at,
  last_error = '',
  updated_at = excluded.updated_at;

-- name: RecordNotificationCursorError :exec
INSERT INTO notification_cursors (
  consumer_id,
  stream_name,
  subject_id,
  last_sequence,
  last_delivery_id,
  last_delivered_at,
  last_error,
  updated_at
) VALUES (
  sqlc.arg(consumer_id),
  sqlc.arg(stream_name),
  sqlc.arg(subject_id),
  0,
  '',
  NULL,
  sqlc.arg(last_error),
  sqlc.arg(updated_at)
)
ON CONFLICT(consumer_id, stream_name, subject_id) DO UPDATE SET
  last_error = excluded.last_error,
  updated_at = excluded.updated_at;

-- name: GetNotificationPreset :one
SELECT
  name,
  events,
  targets,
  filter,
  enabled,
  built_in,
  default_version,
  default_hash,
  user_modified,
  default_update_available,
  created_at,
  updated_at
FROM notification_presets
WHERE name = sqlc.arg(name);

-- name: CreateNotificationPreset :exec
INSERT INTO notification_presets (
  name,
  events,
  targets,
  filter,
  enabled,
  built_in,
  default_version,
  default_hash,
  user_modified,
  default_update_available,
  created_at,
  updated_at
) VALUES (
  sqlc.arg(name),
  sqlc.arg(events),
  sqlc.arg(targets),
  sqlc.arg(filter),
  sqlc.arg(enabled),
  sqlc.arg(built_in),
  sqlc.arg(default_version),
  sqlc.arg(default_hash),
  sqlc.arg(user_modified),
  sqlc.arg(default_update_available),
  sqlc.arg(created_at),
  sqlc.arg(updated_at)
);

-- name: UpdateNotificationPreset :execrows
UPDATE notification_presets
SET events = sqlc.arg(events),
    targets = sqlc.arg(targets),
    filter = sqlc.arg(filter),
    enabled = sqlc.arg(enabled),
    user_modified = sqlc.arg(user_modified),
    default_update_available = sqlc.arg(default_update_available),
    updated_at = sqlc.arg(updated_at)
WHERE name = sqlc.arg(name);

-- name: DeleteNotificationPreset :execrows
DELETE FROM notification_presets
WHERE name = sqlc.arg(name);

-- name: SeedNotificationPresetDefault :exec
INSERT INTO notification_presets (
  name,
  events,
  targets,
  filter,
  enabled,
  built_in,
  default_version,
  default_hash,
  user_modified,
  default_update_available,
  created_at,
  updated_at
) VALUES (
  sqlc.arg(name),
  sqlc.arg(events),
  sqlc.arg(targets),
  sqlc.arg(filter),
  sqlc.arg(enabled),
  1,
  sqlc.arg(default_version),
  sqlc.arg(default_hash),
  0,
  0,
  sqlc.arg(created_at),
  sqlc.arg(updated_at)
)
ON CONFLICT(name) DO UPDATE SET
  events = CASE
    WHEN notification_presets.built_in = 1 AND notification_presets.user_modified = 0
    THEN excluded.events ELSE notification_presets.events END,
  targets = CASE
    WHEN notification_presets.built_in = 1 AND notification_presets.user_modified = 0
    THEN excluded.targets ELSE notification_presets.targets END,
  filter = CASE
    WHEN notification_presets.built_in = 1 AND notification_presets.user_modified = 0
    THEN excluded.filter ELSE notification_presets.filter END,
  enabled = CASE
    WHEN notification_presets.built_in = 1 AND notification_presets.user_modified = 0
    THEN excluded.enabled ELSE notification_presets.enabled END,
  built_in = CASE
    WHEN notification_presets.built_in = 1 THEN 1 ELSE notification_presets.built_in END,
  default_version = CASE
    WHEN notification_presets.built_in = 1 THEN excluded.default_version
    ELSE notification_presets.default_version END,
  default_hash = CASE
    WHEN notification_presets.built_in = 1 THEN excluded.default_hash
    ELSE notification_presets.default_hash END,
  default_update_available = CASE
    WHEN notification_presets.built_in = 1
      AND notification_presets.user_modified = 1
      AND notification_presets.default_hash <> excluded.default_hash
    THEN 1
    WHEN notification_presets.built_in = 1 AND notification_presets.user_modified = 0
    THEN 0
    ELSE notification_presets.default_update_available END,
  updated_at = CASE
    WHEN notification_presets.built_in = 1 THEN excluded.updated_at
    ELSE notification_presets.updated_at END;
