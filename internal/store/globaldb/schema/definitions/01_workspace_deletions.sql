CREATE TABLE workspace_deletion_intents (
    workspace_id  TEXT NOT NULL PRIMARY KEY,
    root_dir      TEXT NOT NULL,
    add_dirs      TEXT NOT NULL,
    name          TEXT NOT NULL,
    default_agent TEXT,
    sandbox_ref   TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    requested_at  TEXT NOT NULL
);

CREATE INDEX idx_workspace_deletion_intents_requested
ON workspace_deletion_intents(requested_at, workspace_id);

CREATE TRIGGER workspace_deletion_intents_registration_guard
BEFORE INSERT ON workspaces
WHEN EXISTS (
    SELECT 1 FROM workspace_deletion_intents WHERE workspace_id = NEW.id
)
BEGIN
    SELECT RAISE(ABORT, 'workspace deletion pending');
END;
