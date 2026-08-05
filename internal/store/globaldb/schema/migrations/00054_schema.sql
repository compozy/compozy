-- +goose Up
-- add column "archived_at" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `archived_at` text NULL;
-- create index "idx_sessions_catalog_archive_recent" to table: "sessions"
CREATE INDEX `idx_sessions_catalog_archive_recent` ON `sessions` (`workspace_id`, `archived_at`, `state`, `updated_at` DESC, `created_at` DESC, `id` DESC);

-- +goose StatementBegin
CREATE TRIGGER trg_sessions_archive_insert_guard
BEFORE INSERT ON sessions
WHEN NEW.archived_at IS NOT NULL AND NEW.state != 'stopped'
BEGIN
    SELECT RAISE(ABORT, 'session is archived');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_sessions_archive_update_guard
BEFORE UPDATE OF state, archived_at ON sessions
WHEN NEW.archived_at IS NOT NULL AND NEW.state != 'stopped'
BEGIN
    SELECT RAISE(ABORT, 'session is archived');
END;
-- +goose StatementEnd
