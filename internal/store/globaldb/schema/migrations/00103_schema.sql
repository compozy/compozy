-- +goose Up
-- +goose StatementBegin
CREATE TRIGGER calls_profile_owner_immutable BEFORE UPDATE OF profile_id ON calls
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER calls_profile_owner_active BEFORE INSERT ON calls BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER call_messages_profile_owner_immutable BEFORE UPDATE OF profile_id ON call_messages
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER call_messages_profile_owner_active BEFORE INSERT ON call_messages BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER call_messages_profile_owner_active;
DROP TRIGGER call_messages_profile_owner_immutable;
DROP TRIGGER calls_profile_owner_active;
DROP TRIGGER calls_profile_owner_immutable;
