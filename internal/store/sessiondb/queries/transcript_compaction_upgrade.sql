-- name: ListMissingUnarchivedCompactionEntries :many
WITH compaction_presence AS (
  SELECT 1 FROM events WHERE type = 'session.compaction_fired' LIMIT 1
)
SELECT e.transcript_entry_key
FROM compaction_presence CROSS JOIN events AS e
WHERE e.archived = 0 AND e.transcript_entry_key <> ''
  AND NOT EXISTS (SELECT 1 FROM transcript_entries AS p WHERE p.entry_key = e.transcript_entry_key)
  AND EXISTS (
    SELECT 1 FROM events AS f
    WHERE f.type = 'session.compaction_fired'
      AND e.sequence BETWEEN json_extract(f.content, '$.raw.from_sequence')
                         AND json_extract(f.content, '$.raw.to_sequence'))
GROUP BY e.transcript_entry_key
ORDER BY MIN(e.sequence);

-- name: GetTranscriptEntryUpgradeCutoff :one
-- A fired event precedes archival and can represent a failed attempt. The last
-- attempt covering every assigned event identifies the successful cut: subsequent
-- compaction spans only inspect unarchived events and cannot cover this entry.
SELECT CAST(COALESCE(MAX(MAX(f.sequence - 1,
                             json_extract(f.content, '$.raw.to_sequence'),
                             bounds.last_sequence)), 0) AS INTEGER) AS cutoff_sequence
FROM events AS f
CROSS JOIN (
  SELECT MIN(assigned.sequence) AS first_sequence, MAX(assigned.sequence) AS last_sequence
  FROM events AS assigned WHERE assigned.transcript_entry_key = sqlc.arg(entry_key)) AS bounds
WHERE f.type = 'session.compaction_fired'
  AND bounds.first_sequence >= json_extract(f.content, '$.raw.from_sequence')
  AND bounds.last_sequence <= json_extract(f.content, '$.raw.to_sequence');

-- name: ListTranscriptKeyedEventsThrough :many
SELECT e.id, e.sequence, e.turn_id, e.type, e.agent_name, e.content, e.archived, e.timestamp, e.transcript_entry_key
FROM events AS e
WHERE e.sequence <= sqlc.arg(through_sequence)
  AND e.transcript_entry_key <> ''
ORDER BY e.sequence ASC;
