-- name: GetTranscriptEntryIdentity :one
SELECT entry_key, kind, logical_id, turn_id, base_message_id,
       COALESCE(message_id, '') AS message_id,
       start_sequence, updated_sequence, complete
FROM transcript_entries
WHERE entry_key = sqlc.arg(entry_key);

-- name: GetTranscriptToolEntryIdentity :one
SELECT e.entry_key, e.kind, e.logical_id, e.turn_id, e.base_message_id,
       COALESCE(e.message_id, '') AS message_id,
       e.start_sequence, e.updated_sequence, e.complete
FROM transcript_tool_routes AS r
JOIN transcript_entries AS e ON e.entry_key = r.entry_key
WHERE r.tool_key = sqlc.arg(tool_key);

-- name: GetLatestAssistantTranscriptIdentity :one
SELECT entry_key, kind, logical_id, turn_id, base_message_id,
       COALESCE(message_id, '') AS message_id,
       start_sequence, updated_sequence, complete
FROM transcript_entries
WHERE kind = sqlc.arg(kind) AND turn_id = sqlc.arg(turn_id)
ORDER BY start_sequence DESC
LIMIT 1;

-- name: GetTranscriptProjectionState :one
SELECT projection_version, generation, COALESCE(active_entry_key, '') AS active_entry_key
FROM transcript_projection_state
WHERE singleton = 1;

-- name: UpdateTranscriptProjectionState :exec
UPDATE transcript_projection_state
SET projection_version = sqlc.arg(projection_version),
    generation = sqlc.arg(generation),
    active_entry_key = NULLIF(CAST(sqlc.arg(active_entry_key) AS TEXT), '')
WHERE singleton = 1;

-- name: UpsertTranscriptEntry :exec
INSERT INTO transcript_entries (
    entry_key, kind, logical_id, turn_id, base_message_id, message_id,
    start_sequence, updated_sequence, complete, message_json, event_type, marker_json
) VALUES (
    sqlc.arg(entry_key), sqlc.arg(kind), sqlc.arg(logical_id), sqlc.arg(turn_id),
    sqlc.arg(base_message_id), NULLIF(CAST(sqlc.arg(message_id) AS TEXT), ''),
    sqlc.arg(start_sequence), sqlc.arg(updated_sequence), sqlc.arg(complete),
    sqlc.narg(message_json), sqlc.arg(event_type), sqlc.narg(marker_json)
)
ON CONFLICT(entry_key) DO UPDATE SET
    logical_id = excluded.logical_id,
    turn_id = excluded.turn_id,
    base_message_id = excluded.base_message_id,
    message_id = excluded.message_id,
    updated_sequence = excluded.updated_sequence,
    complete = excluded.complete,
    message_json = excluded.message_json,
    event_type = excluded.event_type,
    marker_json = excluded.marker_json;

-- name: CountTranscriptMessageIDConflicts :one
SELECT COUNT(*)
FROM transcript_entries
WHERE message_id = CAST(sqlc.arg(message_id) AS TEXT) AND entry_key <> sqlc.arg(entry_key);

-- name: ListEventsForTranscriptEntry :many
SELECT id, sequence, turn_id, type, agent_name, content, archived, timestamp
FROM events
WHERE transcript_entry_key = sqlc.arg(transcript_entry_key)
ORDER BY sequence ASC;

-- name: UpsertTranscriptToolRoute :exec
INSERT INTO transcript_tool_routes (tool_key, entry_key)
VALUES (sqlc.arg(tool_key), sqlc.arg(entry_key))
ON CONFLICT(tool_key) DO UPDATE SET entry_key = excluded.entry_key;

-- name: ListLatestTranscriptEntries :many
SELECT message_json, start_sequence, updated_sequence, event_type, marker_json
FROM transcript_entries
WHERE message_json IS NOT NULL
ORDER BY start_sequence DESC
LIMIT sqlc.arg(row_limit);

-- name: ListTranscriptEntriesBefore :many
SELECT message_json, start_sequence, updated_sequence, event_type, marker_json
FROM transcript_entries
WHERE message_json IS NOT NULL AND start_sequence < sqlc.arg(before_sequence)
ORDER BY start_sequence DESC
LIMIT sqlc.arg(row_limit);
