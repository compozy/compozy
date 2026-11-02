CREATE TABLE agent_instances (
    id TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL REFERENCES profiles(id),
    agent_name TEXT NOT NULL,
    workspace_id TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (state IN ('starting', 'active', 'paused')),
    pause_reason TEXT NOT NULL DEFAULT '',
    main_session_id TEXT REFERENCES sessions(id) ON DELETE SET NULL,
    grant_ceiling TEXT NOT NULL DEFAULT '' CHECK (grant_ceiling IN ('', 'unrestricted')),
    permission_ceiling TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(permission_ceiling)),
    checks_enabled INTEGER NOT NULL DEFAULT 1 CHECK (checks_enabled IN (0, 1)),
    depth INTEGER NOT NULL DEFAULT 0 CHECK (depth IN (0, 1)),
    created_by_kind TEXT NOT NULL,
    created_by_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    archived_at TEXT
);

CREATE UNIQUE INDEX idx_agent_instances_live_identity
    ON agent_instances(profile_id, agent_name, workspace_id) WHERE archived_at IS NULL;
CREATE INDEX idx_agent_instances_main_session ON agent_instances(main_session_id);
CREATE INDEX idx_agent_instances_workspace ON agent_instances(workspace_id);

CREATE TABLE agent_instance_reports (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL REFERENCES agent_instances(id) ON DELETE CASCADE,
    source_key TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL CHECK (kind IN ('proposal', 'failure', 'approval_expired', 'interrupted', 'rotation_failed', 'rotation_deferred')),
    payload TEXT NOT NULL CHECK (json_valid(payload)),
    fingerprint TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'reserved', 'delivered', 'indeterminate')),
    prompt_admission_id TEXT NOT NULL DEFAULT '',
    event_session_id TEXT NOT NULL DEFAULT '',
    event_seq INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    delivered_at TEXT,
    recovery_noted INTEGER NOT NULL DEFAULT 0 CHECK (recovery_noted IN (0, 1))
);

CREATE INDEX idx_agent_instance_reports_history ON agent_instance_reports(instance_id, created_at, id);
CREATE INDEX idx_agent_instance_reports_pending ON agent_instance_reports(instance_id, state, created_at, id);

CREATE INDEX idx_agent_instance_reports_unnoted ON agent_instance_reports(id)
    WHERE state = 'indeterminate' AND recovery_noted = 0;

CREATE TRIGGER agent_instances_workspace_insert_guard
BEFORE INSERT ON agent_instances
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
    SELECT RAISE(ABORT, 'workspace not found');
END;

CREATE TRIGGER agent_instances_workspace_update_guard
BEFORE UPDATE OF workspace_id ON agent_instances
WHEN NEW.workspace_id <> '' AND NOT EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id)
BEGIN
    SELECT RAISE(ABORT, 'workspace not found');
END;

CREATE TABLE agent_instance_wake_state (
    instance_id TEXT PRIMARY KEY REFERENCES agent_instances(id) ON DELETE CASCADE,
    next_wake_at TEXT NOT NULL DEFAULT '',
    last_wake_at TEXT NOT NULL DEFAULT '',
    last_digest TEXT NOT NULL DEFAULT '',
    last_digest_at TEXT NOT NULL DEFAULT '',
    automation_cursor TEXT NOT NULL DEFAULT '',
    quiet_streak INTEGER NOT NULL DEFAULT 0 CHECK (quiet_streak >= 0),
    failure_streak INTEGER NOT NULL DEFAULT 0 CHECK (failure_streak >= 0),
    occurrence_id TEXT NOT NULL DEFAULT '',
    triage_session_id TEXT NOT NULL DEFAULT '',
    occurrence_started_at TEXT NOT NULL DEFAULT '',
    occurrence_cancelled INTEGER NOT NULL DEFAULT 0 CHECK (occurrence_cancelled IN (0, 1))
);

CREATE INDEX idx_agent_instance_wake_state_due ON agent_instance_wake_state(next_wake_at, instance_id);

CREATE TABLE agent_instance_seen (
    instance_id TEXT NOT NULL REFERENCES agent_instances(id) ON DELETE CASCADE,
    operator_id TEXT NOT NULL CHECK (length(operator_id) > 0),
    last_seen_at TEXT NOT NULL,
    PRIMARY KEY (instance_id, operator_id)
);

CREATE TABLE agent_instance_rotation_wait (
    instance_id TEXT PRIMARY KEY REFERENCES agent_instances(id) ON DELETE CASCADE,
    main_session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    started_at TEXT NOT NULL
);
