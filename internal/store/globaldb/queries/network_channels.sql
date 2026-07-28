-- name: CreateNetworkChannel :exec
INSERT INTO network_channels (
  channel, workspace_id, purpose, fanout_policy, coordinator_peer_id,
  created_by, created_at, updated_at
) VALUES (
  sqlc.arg(channel), sqlc.arg(workspace_id), sqlc.arg(purpose), sqlc.arg(fanout_policy),
  sqlc.arg(coordinator_peer_id), sqlc.arg(created_by), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: UpsertNetworkChannel :exec
INSERT INTO network_channels (
  channel, workspace_id, purpose, fanout_policy, coordinator_peer_id,
  created_by, created_at, updated_at
) VALUES (
  sqlc.arg(channel), sqlc.arg(workspace_id), sqlc.arg(purpose), sqlc.arg(fanout_policy),
  sqlc.arg(coordinator_peer_id), sqlc.arg(created_by), sqlc.arg(created_at), sqlc.arg(updated_at)
)
ON CONFLICT(workspace_id, channel) DO UPDATE SET
  purpose = excluded.purpose,
  fanout_policy = excluded.fanout_policy,
  coordinator_peer_id = excluded.coordinator_peer_id,
  updated_at = excluded.updated_at,
  created_by = CASE
    WHEN TRIM(network_channels.created_by) = '' THEN excluded.created_by
    ELSE network_channels.created_by
  END;

-- name: GetNetworkChannel :one
SELECT channel, workspace_id, purpose, fanout_policy, coordinator_peer_id,
       created_by, created_at, updated_at
FROM network_channels
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel);

-- name: PatchNetworkChannel :execrows
UPDATE network_channels SET
  purpose = sqlc.arg(purpose),
  fanout_policy = sqlc.arg(fanout_policy),
  coordinator_peer_id = sqlc.arg(coordinator_peer_id),
  updated_at = sqlc.arg(updated_at)
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel);

-- name: DeleteNetworkChannel :exec
DELETE FROM network_channels
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel);

-- name: EnsureNetworkChannelProjection :exec
INSERT OR IGNORE INTO network_channel_stats (
  workspace_id, channel, message_count, presence_count, historical_participant_count,
  last_activity_at, last_activity_sequence, last_presence_at, last_presence_sequence,
  last_message_id, last_message_sequence, last_message_preview
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(channel), 0, 0, 0, NULL, 0, NULL, 0, '', 0, ''
);

-- name: IncrementNetworkChannelPresence :exec
UPDATE network_channel_stats
SET presence_count = presence_count + 1,
    last_presence_at = sqlc.arg(last_presence_at),
    last_presence_sequence = sqlc.arg(last_presence_sequence)
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel);

-- name: IncrementNetworkChannelMessage :exec
UPDATE network_channel_stats
SET message_count = message_count + 1,
    last_message_preview = sqlc.arg(last_message_preview),
    last_message_id = sqlc.arg(last_message_id),
    last_message_sequence = sqlc.arg(last_message_sequence),
    last_activity_at = sqlc.arg(last_activity_at),
    last_activity_sequence = sqlc.arg(last_activity_sequence)
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel);

-- name: IncrementNetworkChannelKind :exec
INSERT INTO network_channel_kind_counts (workspace_id, channel, kind, message_count)
VALUES (sqlc.arg(workspace_id), sqlc.arg(channel), sqlc.arg(kind), 1)
ON CONFLICT(workspace_id, channel, kind) DO UPDATE SET
  message_count = message_count + 1;

-- name: InsertNetworkChannelParticipant :execrows
INSERT OR IGNORE INTO network_channel_participants (workspace_id, channel, session_id)
VALUES (sqlc.arg(workspace_id), sqlc.arg(channel), sqlc.arg(session_id));

-- name: IncrementNetworkChannelParticipantCount :exec
UPDATE network_channel_stats
SET historical_participant_count = historical_participant_count + 1
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel);

-- name: ListNetworkChannelKindCounts :many
SELECT workspace_id, channel, kind, message_count
FROM network_channel_kind_counts
WHERE workspace_id = sqlc.arg(workspace_id) AND channel = sqlc.arg(channel)
ORDER BY kind ASC;
