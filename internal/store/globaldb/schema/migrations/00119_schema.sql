-- +goose Up
-- apply declarative trigger "extension_inputs_profile_delete" on table "profiles"
-- +goose StatementBegin
CREATE TRIGGER extension_inputs_profile_delete
AFTER DELETE ON profiles
BEGIN
 DELETE FROM extension_inputs WHERE profile = OLD.id;
END;
-- +goose StatementEnd
-- apply declarative trigger "extension_inputs_workspace_delete" on table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER extension_inputs_workspace_delete
AFTER DELETE ON workspaces
BEGIN
 DELETE FROM extension_inputs WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
-- apply declarative trigger "extension_mcp_overrides_profile_delete" on table "profiles"
-- +goose StatementBegin
CREATE TRIGGER extension_mcp_overrides_profile_delete
AFTER DELETE ON profiles
BEGIN
 DELETE FROM extension_mcp_overrides WHERE profile = OLD.id;
END;
-- +goose StatementEnd
-- apply declarative trigger "extension_mcp_overrides_workspace_delete" on table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER extension_mcp_overrides_workspace_delete
AFTER DELETE ON workspaces
BEGIN
 DELETE FROM extension_mcp_overrides WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
