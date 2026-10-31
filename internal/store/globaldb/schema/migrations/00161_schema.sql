-- +goose Up
-- add column "parked_at" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `parked_at` text NULL;
-- add column "park_reason" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `park_reason` text NOT NULL DEFAULT '';
-- add column "park_until" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `park_until` text NULL;
-- add column "later_returned_at" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `later_returned_at` text NULL;
-- create index "idx_sessions_later_due" to table: "sessions"
CREATE INDEX `idx_sessions_later_due` ON `sessions` (`park_until`) WHERE parked_at IS NOT NULL AND park_until IS NOT NULL;
-- apply declarative trigger "trg_sessions_later_insert_guard" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER trg_sessions_later_insert_guard
			BEFORE INSERT ON sessions
			WHEN NEW.parked_at IS NOT NULL AND (NEW.state != 'stopped' OR NEW.archived_at IS NOT NULL)
			BEGIN
				SELECT RAISE(ABORT, 'session is in later');
			END;
-- +goose StatementEnd
-- apply declarative trigger "trg_sessions_later_update_guard" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER trg_sessions_later_update_guard
			BEFORE UPDATE OF state, archived_at, parked_at ON sessions
			WHEN NEW.parked_at IS NOT NULL AND (NEW.state != 'stopped' OR NEW.archived_at IS NOT NULL)
			BEGIN
				SELECT RAISE(ABORT, 'session is in later');
			END;
-- +goose StatementEnd
