-- +goose Up
-- create "session_db_identity" table
CREATE TABLE `session_db_identity` (`singleton` integer NULL, `database_id` text NOT NULL, PRIMARY KEY (`singleton`), CHECK (singleton = 1), CHECK (
			length(database_id) = 32
			AND database_id NOT GLOB '*[^0-9a-f]*'
		));

INSERT INTO session_db_identity (singleton, database_id)
	VALUES (1, lower(hex(randomblob(16))));

-- +goose StatementBegin
CREATE TRIGGER session_db_identity_immutable_update
	BEFORE UPDATE ON session_db_identity
	BEGIN
		SELECT RAISE(ABORT, 'session database identity is immutable');
	END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER session_db_identity_immutable_delete
	BEFORE DELETE ON session_db_identity
	BEGIN
		SELECT RAISE(ABORT, 'session database identity is immutable');
	END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER session_db_identity_immutable_insert
	BEFORE INSERT ON session_db_identity
	WHEN EXISTS (SELECT 1 FROM session_db_identity)
	BEGIN
		SELECT RAISE(ABORT, 'session database identity is immutable');
	END;
-- +goose StatementEnd
