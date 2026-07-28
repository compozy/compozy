CREATE TABLE agent_heartbeat_revisions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name TEXT NOT NULL,
			source_path TEXT NOT NULL,
			operation TEXT NOT NULL CHECK (operation IN ('write', 'delete', 'rollback')),
			previous_digest TEXT,
			new_digest TEXT,
			new_snapshot_id TEXT REFERENCES agent_heartbeat_snapshots(id) ON DELETE SET NULL,
			body TEXT,
			actor_kind TEXT NOT NULL CHECK (actor_kind IN ('user', 'agent', 'extension', 'system')),
			actor_id TEXT NOT NULL,
			created_at TEXT NOT NULL
		);

CREATE TABLE agent_heartbeat_snapshots (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name TEXT NOT NULL,
			source_path TEXT NOT NULL,
			schema_version INTEGER NOT NULL DEFAULT 1,
			digest TEXT NOT NULL,
			config_digest TEXT NOT NULL,
			body TEXT NOT NULL,
			frontmatter_json TEXT NOT NULL,
			resolved_json TEXT NOT NULL,
			diagnostics_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE (workspace_id, agent_name, digest)
		);

CREATE TABLE agent_heartbeat_wake_events (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name TEXT NOT NULL,
			session_id TEXT REFERENCES sessions(id) ON DELETE SET NULL,
			policy_snapshot_id TEXT REFERENCES agent_heartbeat_snapshots(id) ON DELETE SET NULL,
			source TEXT NOT NULL CHECK (source IN ('scheduler', 'manual', 'harness_reentry')),
			result TEXT NOT NULL CHECK (
				result IN ('sent', 'skipped', 'coalesced', 'rate_limited', 'failed')
			),
			reason TEXT NOT NULL CHECK (
				reason IN (
					'wake_sent',
					'heartbeat_disabled',
					'heartbeat_invalid',
					'heartbeat_no_policy',
					'heartbeat_rate_limited',
					'heartbeat_no_eligible_session',
					'cooldown_active',
					'quiet_window',
					'session_not_found',
					'session_unhealthy',
					'session_not_attachable',
					'session_prompt_active',
					'session_prompt_active_race',
					'synthetic_prompt_failed',
					'wake_coalesced'
				)
			),
			synthetic_prompt_id TEXT,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL
		);

CREATE TABLE agent_heartbeat_wake_state (
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			agent_name TEXT NOT NULL,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			policy_snapshot_id TEXT REFERENCES agent_heartbeat_snapshots(id) ON DELETE SET NULL,
			last_wake_at TEXT,
			next_allowed_at TEXT,
			coalesced_count INTEGER NOT NULL DEFAULT 0 CHECK (coalesced_count >= 0),
			last_result TEXT NOT NULL CHECK (
				last_result IN ('sent', 'skipped', 'coalesced', 'rate_limited', 'failed')
			),
			last_reason TEXT CHECK (
				last_reason IS NULL OR last_reason IN (
					'wake_sent',
					'heartbeat_disabled',
					'heartbeat_invalid',
					'heartbeat_no_policy',
					'heartbeat_rate_limited',
					'heartbeat_no_eligible_session',
					'cooldown_active',
					'quiet_window',
					'session_not_found',
					'session_unhealthy',
					'session_not_attachable',
					'session_prompt_active',
					'session_prompt_active_race',
					'synthetic_prompt_failed',
					'wake_coalesced'
				)
			),
			updated_at TEXT NOT NULL,
			PRIMARY KEY (workspace_id, agent_name, session_id)
		);

CREATE INDEX idx_agent_heartbeat_revisions_agent_created
			ON agent_heartbeat_revisions(workspace_id, agent_name, created_at DESC, id DESC);

CREATE INDEX idx_agent_heartbeat_snapshots_agent_created
			ON agent_heartbeat_snapshots(workspace_id, agent_name, created_at DESC, id DESC);

CREATE INDEX idx_agent_heartbeat_wake_events_agent_created
			ON agent_heartbeat_wake_events(workspace_id, agent_name, created_at DESC, id DESC);

CREATE INDEX idx_agent_heartbeat_wake_events_expires
			ON agent_heartbeat_wake_events(expires_at);

CREATE INDEX idx_agent_heartbeat_wake_state_next_allowed
			ON agent_heartbeat_wake_state(next_allowed_at, updated_at DESC);
