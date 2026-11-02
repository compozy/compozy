CREATE TABLE session_work_links (
 session_id TEXT NOT NULL,
 workspace_id TEXT NOT NULL,
 repo_key TEXT NOT NULL CHECK (length(trim(repo_key)) > 0),
 branch TEXT NOT NULL CHECK (length(trim(branch)) > 0),
 pr_number INTEGER CHECK (pr_number IS NULL OR pr_number > 0),
 worktree_id TEXT NOT NULL DEFAULT '',
 worktree_name TEXT NOT NULL DEFAULT '',
 provenance TEXT NOT NULL CHECK (provenance IN ('created', 'user', 'agent')),
 unlinked INTEGER NOT NULL DEFAULT 0 CHECK (unlinked IN (0,1)),
 PRIMARY KEY(session_id, repo_key, branch),
 FOREIGN KEY(workspace_id, session_id) REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_session_work_links_workspace ON session_work_links(workspace_id, repo_key, branch);
