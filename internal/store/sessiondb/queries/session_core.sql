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
