-- +goose Up
-- add column "target_scopes" to table: "session_monitors"
ALTER TABLE `session_monitors` ADD COLUMN `target_scopes` text NOT NULL DEFAULT '[]';

-- Existing session monitors were restricted to their owner's workspace and profile.
-- Preserve that immutable ownership even when a watched session has since been deleted.
UPDATE session_monitors
SET target_scopes = (
  SELECT json_group_array(json_object(
    'session_id', value, 'workspace_id', session_monitors.workspace_id, 'profile_id', session_monitors.profile_id
  ))
  FROM json_each(session_monitors.target_session_ids)
)
WHERE kind = 'session';
