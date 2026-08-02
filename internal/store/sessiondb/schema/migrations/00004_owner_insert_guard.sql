-- +goose Up
-- reject INSERT OR REPLACE and every second owner-row insertion
-- +goose StatementBegin
CREATE TRIGGER session_db_owner_immutable_insert
	BEFORE INSERT ON session_db_owner
	WHEN EXISTS (SELECT 1 FROM session_db_owner)
	BEGIN
		SELECT RAISE(ABORT, 'session database owner is immutable');
	END;
-- +goose StatementEnd
