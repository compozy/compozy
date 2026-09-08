-- +goose Up
-- Backfill display facts from the authoritative event ledger without rerouting
-- entries or changing event bytes, identities, archives, or sequence fences.
UPDATE transcript_entries
SET message_json = json_set(message_json, '$.metadata.timestamp', (
    SELECT timestamp FROM events WHERE sequence = transcript_entries.start_sequence
))
WHERE kind IN ('user', 'system') AND message_json IS NOT NULL
  AND json_extract(message_json, '$.metadata.timestamp') IS NULL
  AND EXISTS (SELECT 1 FROM events WHERE sequence = transcript_entries.start_sequence);

WITH tool_parts AS (
    SELECT t.entry_key, p.key AS part_index, p.value AS part_json,
           (SELECT e.content FROM events AS e
            WHERE e.transcript_entry_key = t.entry_key AND e.type = 'tool_result'
              AND json_extract(e.content, '$.tool_call_id') = json_extract(p.value, '$.toolCallId')
            ORDER BY e.sequence DESC LIMIT 1) AS result_json
    FROM transcript_entries AS t, json_each(t.message_json, '$.parts') AS p
    WHERE t.kind = 'assistant' AND t.message_json IS NOT NULL
      AND json_extract(p.value, '$.state') IN ('output-available', 'output-error')
      AND json_extract(p.value, '$.toolCallId') IS NOT NULL
), named_parts AS (
    SELECT entry_key, part_index, part_json,
           COALESCE(NULLIF(trim(json_extract(result_json, '$.tool_name')), ''),
                    NULLIF(trim(json_extract(result_json, '$.title')), '')) AS tool_name,
           NULLIF(trim(json_extract(result_json, '$.title')), '') AS title
    FROM tool_parts WHERE result_json IS NOT NULL
), updated_parts AS (
    SELECT entry_key, part_index,
           json_remove(json_set(part_json,
               '$.type', 'tool-' || tool_name,
               '$.title', COALESCE(title, json_extract(part_json, '$.title'))), '$.toolName') AS part_json
    FROM named_parts WHERE tool_name IS NOT NULL
)
UPDATE transcript_entries AS t
SET message_json = json_set(message_json, '$.parts', json((
    SELECT json_group_array(json(part_json)) FROM (
        SELECT COALESCE(u.part_json, p.value) AS part_json
        FROM json_each(t.message_json, '$.parts') AS p
        LEFT JOIN updated_parts AS u ON u.entry_key = t.entry_key AND u.part_index = p.key
        ORDER BY p.key
    )
)))
WHERE EXISTS (SELECT 1 FROM updated_parts WHERE entry_key = t.entry_key);
