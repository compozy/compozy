CREATE TABLE user_scope_migrations (
    source_workspace_id TEXT PRIMARY KEY,
    source_root TEXT NOT NULL,
    disposition TEXT NOT NULL CHECK (disposition IN ('migrate', 'keep_project')),
    profile_ids TEXT NOT NULL CHECK (json_valid(profile_ids)),
    sql_done_at TEXT,
    journal_state TEXT NOT NULL DEFAULT 'pending' CHECK (journal_state IN ('pending', 'copied', 'verified', 'skipped')),
    layout_state TEXT NOT NULL DEFAULT 'pending' CHECK (layout_state IN ('pending', 'copied', 'verified', 'skipped')),
    registration_removed_at TEXT
);

CREATE TABLE user_scope_migration_archive (
    source_workspace_id TEXT NOT NULL,
    table_name TEXT NOT NULL,
    row_json TEXT NOT NULL CHECK (json_valid(row_json)),
    reason TEXT NOT NULL,
    archived_at TEXT NOT NULL
);

CREATE TABLE workspace_name_aliases (
    old_name TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    created_at TEXT NOT NULL
);
