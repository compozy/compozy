-- An open move of a session to another profile. The row is written before the
-- source is archived and deleted once the move completes or rolls back, so boot
-- reconciliation can settle a move a daemon stop interrupted. The worktree ids and
-- the copy's derive key are chosen up front: every step's outcome is then readable
-- from durable state, without recording which step ran last.
CREATE TABLE session_profile_moves (
 session_id TEXT NOT NULL,
 workspace_id TEXT NOT NULL,
 profile_id TEXT NOT NULL,
 target_profile_id TEXT NOT NULL,
 copy_key TEXT NOT NULL CHECK (length(trim(copy_key)) > 0),
 source_worktree_id TEXT NOT NULL DEFAULT '',
 branch TEXT NOT NULL DEFAULT '',
 head_sha TEXT NOT NULL DEFAULT '',
 target_worktree_id TEXT NOT NULL DEFAULT '',
 restore_worktree_id TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 PRIMARY KEY(session_id),
 FOREIGN KEY(workspace_id, session_id) REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
);
