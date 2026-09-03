-- +goose Up
-- create "workspace_deletion_intents" table
CREATE TABLE `workspace_deletion_intents` (`workspace_id` text NOT NULL, `root_dir` text NOT NULL, `add_dirs` text NOT NULL, `name` text NOT NULL, `default_agent` text NULL, `sandbox_ref` text NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, `requested_at` text NOT NULL, PRIMARY KEY (`workspace_id`));
-- create index "idx_workspace_deletion_intents_requested" to table: "workspace_deletion_intents"
CREATE INDEX `idx_workspace_deletion_intents_requested` ON `workspace_deletion_intents` (`requested_at`, `workspace_id`);
-- apply declarative trigger "workspace_deletion_intents_registration_guard" on table "workspaces"
-- +goose StatementBegin
CREATE TRIGGER workspace_deletion_intents_registration_guard
BEFORE INSERT ON workspaces
WHEN EXISTS (
    SELECT 1 FROM workspace_deletion_intents WHERE workspace_id = NEW.id
)
BEGIN
    SELECT RAISE(ABORT, 'workspace deletion pending');
END;
-- +goose StatementEnd
