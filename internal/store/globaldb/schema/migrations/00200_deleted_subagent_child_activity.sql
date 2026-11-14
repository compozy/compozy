-- +goose Up
UPDATE session_subagents SET child_activity = 'stopped'
WHERE origin = 'delegated' AND child_session_id IS NULL
 AND child_activity IN ('working','idle')
 AND status IN ('completed','failed','canceled','interrupted');

-- +goose Down
-- Deleted children cannot regain their previous liveness.
