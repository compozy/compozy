CREATE TABLE event_summary_outbox (
            seq INTEGER PRIMARY KEY AUTOINCREMENT,
			id                     TEXT NOT NULL UNIQUE,
			profile_id             TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			session_id             TEXT NOT NULL DEFAULT '',
			workspace_id           TEXT NOT NULL DEFAULT '',
			worktree_id            TEXT NOT NULL DEFAULT '',
			type                   TEXT NOT NULL,
			agent_name             TEXT NOT NULL DEFAULT '',
			content_json           TEXT NOT NULL DEFAULT '',
			task_id                TEXT NOT NULL DEFAULT '',
			run_id                 TEXT NOT NULL DEFAULT '',
			workflow_id            TEXT NOT NULL DEFAULT '',
			claim_token_hash       TEXT NOT NULL DEFAULT '',
			lease_until            TEXT NOT NULL DEFAULT '',
			coordinator_session_id TEXT NOT NULL DEFAULT '',
			scheduler_reason       TEXT NOT NULL DEFAULT '',
			hook_event             TEXT NOT NULL DEFAULT '',
			hook_name              TEXT NOT NULL DEFAULT '',
			actor_kind             TEXT NOT NULL DEFAULT '',
			actor_id               TEXT NOT NULL DEFAULT '',
			release_reason         TEXT NOT NULL DEFAULT '',
			parent_session_id      TEXT NOT NULL DEFAULT '',
			root_session_id        TEXT NOT NULL DEFAULT '',
			spawn_depth            INTEGER NOT NULL DEFAULT 0,
			summary                TEXT,
			timestamp              TEXT NOT NULL
		, provider TEXT NOT NULL DEFAULT '', outcome TEXT NOT NULL DEFAULT 'info');


CREATE TRIGGER event_summary_outbox_profile_owner_active BEFORE INSERT ON event_summary_outbox BEGIN
 SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
 SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
