-- +goose Up
ALTER TABLE conversation_rewind_state ADD COLUMN baseline_stale INTEGER NOT NULL DEFAULT 0 CHECK(baseline_stale IN (0,1));

UPDATE conversation_rewind_state
SET baseline_stale = 1
WHERE EXISTS (
    SELECT 1 FROM events e
    WHERE e.archived = 1 AND e.sequence <= conversation_rewind_state.covered_through_sequence
      AND EXISTS (
        SELECT 1 FROM events f
        WHERE f.type = 'session.compaction_fired'
          AND e.sequence BETWEEN json_extract(f.content, '$.raw.from_sequence')
                             AND json_extract(f.content, '$.raw.to_sequence'))
      AND NOT EXISTS (
        SELECT 1 FROM conversation_rewind_receipts r
        WHERE e.sequence BETWEEN r.archived_from_sequence AND r.archived_to_sequence)
);

UPDATE events SET archived = 0
WHERE archived = 1
  AND EXISTS (
    SELECT 1 FROM events f
    WHERE f.type = 'session.compaction_fired'
      AND events.sequence BETWEEN json_extract(f.content, '$.raw.from_sequence')
                              AND json_extract(f.content, '$.raw.to_sequence'))
  AND NOT EXISTS (
    SELECT 1 FROM conversation_rewind_receipts r
    WHERE events.sequence BETWEEN r.archived_from_sequence AND r.archived_to_sequence);
