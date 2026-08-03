-- +goose Up
-- +goose StatementBegin
CREATE TRIGGER extension_env_bindings_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM extension_env_bindings WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
