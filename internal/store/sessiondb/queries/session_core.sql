-- name: InitializeTranscriptProjectionState :exec
INSERT OR IGNORE INTO transcript_projection_state (
    singleton, projection_version, generation, active_entry_key
) VALUES (1, sqlc.arg(projection_version), 0, NULL);

-- name: UpsertTokenUsage :exec
INSERT INTO token_usage (
    turn_id, input_tokens, output_tokens, total_tokens, thought_tokens,
    cache_read_tokens, cache_write_tokens, context_used, context_size,
    cost_amount, cost_currency, timestamp
) VALUES (
    sqlc.arg(turn_id), sqlc.narg(input_tokens), sqlc.narg(output_tokens),
    sqlc.narg(total_tokens), sqlc.narg(thought_tokens), sqlc.narg(cache_read_tokens),
    sqlc.narg(cache_write_tokens), sqlc.narg(context_used), sqlc.narg(context_size),
    sqlc.narg(cost_amount), sqlc.narg(cost_currency), sqlc.arg(timestamp)
)
ON CONFLICT(turn_id) DO UPDATE SET
    input_tokens = COALESCE(excluded.input_tokens, token_usage.input_tokens),
    output_tokens = COALESCE(excluded.output_tokens, token_usage.output_tokens),
    total_tokens = COALESCE(excluded.total_tokens, token_usage.total_tokens),
    thought_tokens = COALESCE(excluded.thought_tokens, token_usage.thought_tokens),
    cache_read_tokens = COALESCE(excluded.cache_read_tokens, token_usage.cache_read_tokens),
    cache_write_tokens = COALESCE(excluded.cache_write_tokens, token_usage.cache_write_tokens),
    context_used = COALESCE(excluded.context_used, token_usage.context_used),
    context_size = COALESCE(excluded.context_size, token_usage.context_size),
    cost_amount = COALESCE(excluded.cost_amount, token_usage.cost_amount),
    cost_currency = COALESCE(excluded.cost_currency, token_usage.cost_currency),
    timestamp = excluded.timestamp;

-- name: ListTokenUsage :many
SELECT turn_id, input_tokens, output_tokens, total_tokens, thought_tokens,
       cache_read_tokens, cache_write_tokens, context_used, context_size,
       cost_amount, cost_currency, timestamp
FROM token_usage
ORDER BY timestamp ASC, turn_id ASC;

-- name: InsertHookRun :exec
INSERT INTO hook_runs (
    id, hook_name, event, source, mode, duration_ns, outcome, dispatch_depth,
    patch_applied, error, required, recorded_at
) VALUES (
    sqlc.arg(id), sqlc.arg(hook_name), sqlc.arg(event), sqlc.arg(source),
    sqlc.arg(mode), sqlc.arg(duration_ns), sqlc.arg(outcome), sqlc.arg(dispatch_depth),
    sqlc.narg(patch_applied), sqlc.narg(error), sqlc.arg(required), sqlc.arg(recorded_at)
);

-- name: MaxEventSequence :one
SELECT CAST(COALESCE(MAX(sequence), 0) AS INTEGER) FROM events;

-- name: MaxActiveEventSequence :one
SELECT CAST(COALESCE(MAX(sequence), 0) AS INTEGER) FROM events WHERE archived = 0;

-- name: GetEventByID :one
SELECT id, sequence, turn_id, type, agent_name, content, archived, timestamp
FROM events
WHERE id = sqlc.arg(id);

-- name: InsertEvent :exec
INSERT INTO events (
    id, sequence, turn_id, type, agent_name, content, archived, timestamp, transcript_entry_key
) VALUES (
    sqlc.arg(id), sqlc.arg(sequence), sqlc.arg(turn_id), sqlc.arg(type),
    sqlc.arg(agent_name), sqlc.arg(content), sqlc.arg(archived), sqlc.arg(timestamp),
    sqlc.arg(transcript_entry_key)
);

-- name: ArchiveEventRange :execrows
UPDATE events
SET archived = 1
WHERE archived = 0
  AND sequence >= sqlc.arg(from_sequence)
  AND sequence <= sqlc.arg(to_sequence);

-- name: ClearHookRuns :exec
DELETE FROM hook_runs;

-- name: ClearTokenUsage :exec
DELETE FROM token_usage;

-- name: ClearTranscriptToolRoutes :exec
DELETE FROM transcript_tool_routes;

-- name: ClearTranscriptEntries :exec
DELETE FROM transcript_entries;

-- name: ClearEvents :exec
DELETE FROM events;

-- name: AdvanceTranscriptProjectionGeneration :exec
UPDATE transcript_projection_state
SET generation = generation + 1, active_entry_key = NULL
WHERE singleton = 1;

-- name: GetConversationRewindTarget :one
SELECT kind, message_id, start_sequence, complete, message_json
FROM transcript_entries
WHERE message_id = sqlc.arg(message_id)
  AND EXISTS (
    SELECT 1 FROM events
    WHERE events.sequence = transcript_entries.start_sequence
      AND events.archived = 0
  );

-- name: CountArchivedTranscriptEventsBeforeSequence :one
SELECT COUNT(*)
FROM events
JOIN transcript_entries
  ON transcript_entries.entry_key = events.transcript_entry_key
WHERE events.archived = 1
  AND transcript_entries.start_sequence < sqlc.arg(before_sequence);

-- name: DeleteTranscriptToolRoutesFromSequence :exec
DELETE FROM transcript_tool_routes
WHERE entry_key IN (
    SELECT entry_key FROM transcript_entries WHERE start_sequence >= sqlc.arg(from_sequence)
);

-- name: DeleteTranscriptEntriesFromSequence :exec
DELETE FROM transcript_entries
WHERE start_sequence >= sqlc.arg(from_sequence);

-- name: UpsertConversationRewindState :exec
INSERT INTO conversation_rewind_state (
    singleton, target_message_id, covered_through_sequence, messages_json, updated_at
) VALUES (
    1, sqlc.arg(target_message_id), sqlc.arg(covered_through_sequence),
    sqlc.arg(messages_json), sqlc.arg(updated_at)
)
ON CONFLICT(singleton) DO UPDATE SET
    target_message_id = excluded.target_message_id,
    covered_through_sequence = excluded.covered_through_sequence,
    messages_json = excluded.messages_json,
    updated_at = excluded.updated_at;

-- name: GetConversationRewindState :one
SELECT target_message_id, covered_through_sequence, messages_json, updated_at
FROM conversation_rewind_state
WHERE singleton = 1;

-- name: GetConversationRewindReceipt :one
SELECT idempotency_key, request_hash, target_message_id,
       archived_from_sequence, archived_to_sequence, archived_event_count,
       generation, max_sequence, transcript_epoch, draft_text, created_at
FROM conversation_rewind_receipts
WHERE idempotency_key = sqlc.arg(idempotency_key);

-- name: InsertConversationRewindReceipt :exec
INSERT INTO conversation_rewind_receipts (
    idempotency_key, request_hash, target_message_id,
    archived_from_sequence, archived_to_sequence, archived_event_count,
    generation, max_sequence, transcript_epoch, draft_text, created_at
) VALUES (
    sqlc.arg(idempotency_key), sqlc.arg(request_hash), sqlc.arg(target_message_id),
    sqlc.arg(archived_from_sequence), sqlc.arg(archived_to_sequence),
    sqlc.arg(archived_event_count), sqlc.arg(generation), sqlc.arg(max_sequence),
    sqlc.arg(transcript_epoch),
    sqlc.arg(draft_text), sqlc.arg(created_at)
);

-- name: ClearConversationRewindState :exec
DELETE FROM conversation_rewind_state;

-- name: ClearConversationRewindReceipts :exec
DELETE FROM conversation_rewind_receipts;
