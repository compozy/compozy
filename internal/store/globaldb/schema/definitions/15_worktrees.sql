CREATE TABLE worktrees (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		branch TEXT NOT NULL DEFAULT '',
		path TEXT NOT NULL,
		git_dir TEXT NOT NULL DEFAULT '',
		state TEXT NOT NULL CHECK (
			state IN ('pending', 'ready', 'failed', 'removing', 'missing', 'removed', 'dismissed')
		),
		pending_phase TEXT NOT NULL DEFAULT '' CHECK (
			pending_phase IN ('', 'branch', 'checkout', 'copy', 'setup')
		),
		origin TEXT NOT NULL CHECK (origin IN ('manual', 'per_run', 'adopted')),
		setup_state TEXT NOT NULL DEFAULT 'none' CHECK (setup_state IN ('none', 'ok', 'failed')),
		setup_error TEXT NOT NULL DEFAULT '',
		base_ref TEXT NOT NULL DEFAULT '',
		created_branch INTEGER NOT NULL DEFAULT 0 CHECK (created_branch IN (0, 1)),
		run_namespace TEXT NOT NULL DEFAULT '',
		created_head TEXT NOT NULL DEFAULT '',
		run_id TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		UNIQUE (workspace_id, id)
	);

CREATE INDEX idx_worktrees_workspace_state
		ON worktrees(workspace_id, state);

CREATE UNIQUE INDEX idx_worktrees_reserved_name
		ON worktrees(workspace_id, name)
		WHERE state <> 'dismissed';

CREATE UNIQUE INDEX idx_worktrees_live_path
		ON worktrees(path)
		WHERE state IN ('pending', 'ready', 'removing');

CREATE TABLE worktree_status (
		worktree_id TEXT PRIMARY KEY REFERENCES worktrees(id) ON DELETE CASCADE,
		branch TEXT,
		detached INTEGER CHECK (detached IS NULL OR detached IN (0, 1)),
		head_sha TEXT,
		dirty_files INTEGER CHECK (dirty_files IS NULL OR dirty_files >= 0),
		insertions INTEGER CHECK (insertions IS NULL OR insertions >= 0),
		deletions INTEGER CHECK (deletions IS NULL OR deletions >= 0),
		has_upstream INTEGER CHECK (has_upstream IS NULL OR has_upstream IN (0, 1)),
		ahead INTEGER CHECK (ahead IS NULL OR ahead >= 0),
		behind INTEGER CHECK (behind IS NULL OR behind >= 0),
		read_error TEXT NOT NULL DEFAULT '',
		refreshed_at TEXT
	);

CREATE TABLE worktree_forge_status (
		worktree_id TEXT PRIMARY KEY REFERENCES worktrees(id) ON DELETE CASCADE,
		provider TEXT NOT NULL DEFAULT '',
		pr_number INTEGER CHECK (pr_number IS NULL OR pr_number > 0),
		pr_state TEXT CHECK (pr_state IS NULL OR pr_state IN ('open', 'closed', 'merged')),
		pr_url TEXT NOT NULL DEFAULT '',
		merged INTEGER CHECK (merged IS NULL OR merged IN (0, 1)),
		fetched_at TEXT
	);

CREATE TABLE worktree_exit_ops (
		op_id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		worktree_id TEXT NOT NULL,
		action TEXT NOT NULL CHECK (action IN ('commit', 'commit_push', 'push', 'open_pr')),
		state TEXT NOT NULL CHECK (state IN ('running', 'completed', 'failed', 'canceled')),
		started_at TEXT NOT NULL,
		finished_at TEXT,
		FOREIGN KEY (workspace_id, worktree_id)
			REFERENCES worktrees(workspace_id, id) ON DELETE CASCADE
	);

CREATE UNIQUE INDEX idx_worktree_exit_ops_active
		ON worktree_exit_ops(worktree_id)
		WHERE state = 'running';
