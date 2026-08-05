-- +goose Up
-- create "session_db_owner" table
CREATE TABLE `session_db_owner` (`singleton` integer NULL, `session_id` text NOT NULL, `workspace_id` text NOT NULL, PRIMARY KEY (`singleton`), CHECK (singleton = 1), CHECK (length(trim(session_id)) > 0), CHECK (length(trim(workspace_id)) > 0));
-- +goose StatementBegin
CREATE TRIGGER session_db_owner_immutable_update
	BEFORE UPDATE ON session_db_owner
	BEGIN
		SELECT RAISE(ABORT, 'session database owner is immutable');
	END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER session_db_owner_immutable_delete
	BEFORE DELETE ON session_db_owner
	BEGIN
		SELECT RAISE(ABORT, 'session database owner is immutable');
	END;
-- +goose StatementEnd
