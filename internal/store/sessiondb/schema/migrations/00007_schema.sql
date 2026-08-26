-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_session_db_owner" table
CREATE TABLE `new_session_db_owner` (`singleton` integer NULL, `session_id` text NOT NULL, `workspace_id` text NOT NULL, PRIMARY KEY (`singleton`), CHECK (singleton = 1), CHECK (length(trim(session_id)) > 0));
-- copy rows from old table "session_db_owner" to new temporary table "new_session_db_owner"
INSERT INTO `new_session_db_owner` (`singleton`, `session_id`, `workspace_id`) SELECT `singleton`, `session_id`, `workspace_id` FROM `session_db_owner`;
-- drop trigger "session_db_owner_immutable_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `session_db_owner_immutable_delete`;
-- drop trigger "session_db_owner_immutable_insert" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `session_db_owner_immutable_insert`;
-- drop trigger "session_db_owner_immutable_update" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `session_db_owner_immutable_update`;
-- drop "session_db_owner" table after copying rows
DROP TABLE `session_db_owner`;
-- rename temporary table "new_session_db_owner" to "session_db_owner"
ALTER TABLE `new_session_db_owner` RENAME TO `session_db_owner`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- recreate trigger "session_db_owner_immutable_delete" after rebuilding table "session_db_owner"
-- +goose StatementBegin
CREATE TRIGGER session_db_owner_immutable_delete
		BEFORE DELETE ON session_db_owner
		BEGIN
			SELECT RAISE(ABORT, 'session database owner is immutable');
		END;
-- +goose StatementEnd
-- recreate trigger "session_db_owner_immutable_insert" after rebuilding table "session_db_owner"
-- +goose StatementBegin
CREATE TRIGGER session_db_owner_immutable_insert
		BEFORE INSERT ON session_db_owner
		WHEN EXISTS (SELECT 1 FROM session_db_owner)
		BEGIN
			SELECT RAISE(ABORT, 'session database owner is immutable');
		END;
-- +goose StatementEnd
-- recreate trigger "session_db_owner_immutable_update" after rebuilding table "session_db_owner"
-- +goose StatementBegin
CREATE TRIGGER session_db_owner_immutable_update
		BEFORE UPDATE ON session_db_owner
		BEGIN
			SELECT RAISE(ABORT, 'session database owner is immutable');
		END;
-- +goose StatementEnd
