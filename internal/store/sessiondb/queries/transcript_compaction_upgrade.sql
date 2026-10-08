-- name: ListMissingUnarchivedCompactionEntries :many
SELECT e.transcript_entry_key
FROM events AS e
WHERE e.archived = 0 AND e.transcript_entry_key <> ''
  AND NOT EXISTS (SELECT 1 FROM transcript_entries AS p WHERE p.entry_key = e.transcript_entry_key)
  AND EXISTS (
    SELECT 1 FROM events AS f
    WHERE f.type = 'session.compaction_fired'
      AND e.sequence BETWEEN json_extract(f.content, '$.raw.from_sequence')
                         AND json_extract(f.content, '$.raw.to_sequence'))
GROUP BY e.transcript_entry_key
ORDER BY MIN(e.sequence);

-- name: GetNextTranscriptBoundaryEventForUpgrade :one
SELECT e.id, e.sequence, e.turn_id, e.type, e.agent_name, e.content, e.archived, e.timestamp, e.transcript_entry_key
FROM events AS e
WHERE e.sequence > (SELECT MAX(a.sequence) FROM events AS a WHERE a.transcript_entry_key = sqlc.arg(entry_key))
  AND e.transcript_entry_key <> ''
ORDER BY e.sequence ASC
LIMIT 1;
