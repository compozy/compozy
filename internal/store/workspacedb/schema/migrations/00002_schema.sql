-- +goose Up
-- create "terminal_recordings" table
CREATE TABLE `terminal_recordings` (`id` text NULL, `terminal_id` text NOT NULL, `profile_id` text NOT NULL, `digest` text NOT NULL, `path` text NOT NULL, `started_at` integer NOT NULL, `stopped_at` integer NULL, `bytes` integer NOT NULL, `expires_at` integer NOT NULL, PRIMARY KEY (`id`));
-- create index "idx_terminal_recordings_terminal_started" to table: "terminal_recordings"
CREATE INDEX `idx_terminal_recordings_terminal_started` ON `terminal_recordings` (`terminal_id`, `started_at` DESC, `id` DESC);
-- create index "idx_terminal_recordings_profile_started" to table: "terminal_recordings"
CREATE INDEX `idx_terminal_recordings_profile_started` ON `terminal_recordings` (`profile_id`, `started_at` DESC, `id` DESC);
-- create index "idx_terminal_recordings_expires" to table: "terminal_recordings"
CREATE INDEX `idx_terminal_recordings_expires` ON `terminal_recordings` (`expires_at`, `id`);
-- create "terminal_commands" table
CREATE TABLE `terminal_commands` (`id` text NULL, `terminal_id` text NULL, `profile_id` text NOT NULL, `actor_kind` text NOT NULL, `actor_id` text NOT NULL, `session_id` text NULL, `run_id` text NULL, `command` text NOT NULL, `argv_digest` text NULL, `cwd` text NOT NULL, `started_at` integer NOT NULL, `duration_ms` integer NULL, `exit_code` integer NULL, `exit_signal` text NULL, `exit_cause` text NOT NULL, `detected_by` text NOT NULL, `approval` text NOT NULL, `output_bytes` integer NOT NULL, `truncated` integer NOT NULL, `recording_id` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`recording_id`) REFERENCES `terminal_recordings` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "idx_terminal_commands_terminal_started" to table: "terminal_commands"
CREATE INDEX `idx_terminal_commands_terminal_started` ON `terminal_commands` (`terminal_id`, `started_at` DESC, `id` DESC);
-- create index "idx_terminal_commands_profile_started" to table: "terminal_commands"
CREATE INDEX `idx_terminal_commands_profile_started` ON `terminal_commands` (`profile_id`, `started_at` DESC, `id` DESC);
-- create index "idx_terminal_commands_started" to table: "terminal_commands"
CREATE INDEX `idx_terminal_commands_started` ON `terminal_commands` (`started_at` DESC, `id` DESC);
-- create "terminal_artifacts" table
CREATE TABLE `terminal_artifacts` (`id` text NULL, `terminal_id` text NULL, `command_id` text NOT NULL, `profile_id` text NOT NULL, `digest` text NOT NULL, `path` text NOT NULL, `bytes` integer NOT NULL, `expires_at` integer NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`command_id`) REFERENCES `terminal_commands` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "idx_terminal_artifacts_command" to table: "terminal_artifacts"
CREATE INDEX `idx_terminal_artifacts_command` ON `terminal_artifacts` (`command_id`, `id`);
-- create index "idx_terminal_artifacts_profile" to table: "terminal_artifacts"
CREATE INDEX `idx_terminal_artifacts_profile` ON `terminal_artifacts` (`profile_id`, `id`);
-- create index "idx_terminal_artifacts_expires" to table: "terminal_artifacts"
CREATE INDEX `idx_terminal_artifacts_expires` ON `terminal_artifacts` (`expires_at`, `id`);
-- apply declarative trigger "terminal_artifacts_profile_owner_immutable" on table "terminal_artifacts"
-- +goose StatementBegin
CREATE TRIGGER terminal_artifacts_profile_owner_immutable
BEFORE UPDATE OF profile_id ON terminal_artifacts
WHEN NEW.profile_id <> OLD.profile_id
BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- apply declarative trigger "terminal_commands_profile_owner_immutable" on table "terminal_commands"
-- +goose StatementBegin
CREATE TRIGGER terminal_commands_profile_owner_immutable
BEFORE UPDATE OF profile_id ON terminal_commands
WHEN NEW.profile_id <> OLD.profile_id
BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- apply declarative trigger "terminal_recordings_profile_owner_immutable" on table "terminal_recordings"
-- +goose StatementBegin
CREATE TRIGGER terminal_recordings_profile_owner_immutable
BEFORE UPDATE OF profile_id ON terminal_recordings
WHEN NEW.profile_id <> OLD.profile_id
BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
