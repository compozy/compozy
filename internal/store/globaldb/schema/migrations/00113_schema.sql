-- +goose Up
-- create "extension_installations" table
CREATE TABLE `extension_installations` (`extension_name` text NOT NULL, `profile_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL DEFAULT '', `created_at` text NOT NULL, PRIMARY KEY (`extension_name`, `profile_id`, `workspace_id`), CONSTRAINT `0` FOREIGN KEY (`extension_name`) REFERENCES `extensions` (`name`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (profile_id = trim(profile_id) AND workspace_id = trim(workspace_id)));
-- apply declarative trigger "extension_installations_default_insert" on table "extensions"
-- +goose StatementBegin
CREATE TRIGGER extension_installations_default_insert
AFTER INSERT ON extensions
BEGIN
 INSERT INTO extension_installations (extension_name, profile_id, workspace_id, created_at)
 VALUES (NEW.name, '', '', NEW.installed_at);
END;
-- +goose StatementEnd
-- apply declarative trigger "extension_installations_profile_delete" on table "profiles"
-- +goose StatementBegin
CREATE TRIGGER extension_installations_profile_delete
AFTER DELETE ON profiles
BEGIN
 DELETE FROM extension_installations WHERE profile_id = OLD.id;
END;
-- +goose StatementEnd
-- apply declarative trigger "extension_installations_profile_insert" on table "extension_installations"
-- +goose StatementBegin
CREATE TRIGGER extension_installations_profile_insert
BEFORE INSERT ON extension_installations
WHEN NEW.profile_id <> '' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id)
BEGIN
 SELECT RAISE(ABORT, 'profile_not_found');
END;
-- +goose StatementEnd
-- apply declarative trigger "extension_installations_workspace_delete" on table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER extension_installations_workspace_delete
AFTER DELETE ON workspaces
BEGIN
 DELETE FROM extension_installations WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
-- apply declarative trigger "extension_installations_workspace_insert" on table "extension_installations"
-- +goose StatementBegin
CREATE TRIGGER extension_installations_workspace_insert
BEFORE INSERT ON extension_installations
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
 SELECT RAISE(ABORT, 'workspace_not_found');
END;
-- +goose StatementEnd

-- Preserve the original global/all-profiles authority of every installed package.
INSERT INTO extension_installations (extension_name, profile_id, workspace_id, created_at)
SELECT name, '', '', installed_at FROM extensions;
