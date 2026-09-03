-- +goose Up
-- create index "idx_terminal_recordings_path" to table: "terminal_recordings"
CREATE INDEX `idx_terminal_recordings_path` ON `terminal_recordings` (`path`, `id`);
-- create index "idx_terminal_commands_recording" to table: "terminal_commands"
CREATE INDEX `idx_terminal_commands_recording` ON `terminal_commands` (`recording_id`);
-- create index "idx_terminal_artifacts_path" to table: "terminal_artifacts"
CREATE INDEX `idx_terminal_artifacts_path` ON `terminal_artifacts` (`path`, `id`);
