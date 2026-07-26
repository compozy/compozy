CREATE TABLE agent_soul_revisions (
			id               TEXT PRIMARY KEY,
			workspace_id     TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name       TEXT NOT NULL,
			source_path      TEXT NOT NULL,
			action           TEXT NOT NULL CHECK (action IN ('put', 'delete', 'rollback')),
			previous_digest  TEXT NOT NULL DEFAULT '',
			new_digest       TEXT NOT NULL DEFAULT '',
			body             TEXT NOT NULL DEFAULT '',
			diagnostics_json TEXT NOT NULL DEFAULT '[]',
			actor_kind       TEXT NOT NULL DEFAULT '',
			actor_id         TEXT NOT NULL DEFAULT '',
			origin_kind      TEXT NOT NULL DEFAULT '',
			origin_ref       TEXT NOT NULL DEFAULT '',
			created_at       TEXT NOT NULL
		);

CREATE TABLE agent_soul_snapshots (
			id            TEXT PRIMARY KEY,
			workspace_id  TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name    TEXT NOT NULL,
			source_path   TEXT NOT NULL,
			digest        TEXT NOT NULL,
			profile_json  TEXT NOT NULL DEFAULT '{}',
			body          TEXT NOT NULL DEFAULT '',
			truncated     INTEGER NOT NULL DEFAULT 0 CHECK (truncated IN (0, 1)),
			created_at    TEXT NOT NULL,
			UNIQUE (workspace_id, agent_name, digest)
		);

CREATE INDEX idx_agent_soul_revisions_agent
			ON agent_soul_revisions(workspace_id, agent_name, created_at DESC);

CREATE INDEX idx_agent_soul_snapshots_agent
			ON agent_soul_snapshots(workspace_id, agent_name, created_at DESC);
