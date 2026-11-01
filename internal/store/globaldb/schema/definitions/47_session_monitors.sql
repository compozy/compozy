-- Daemon-native monitors survive a daemon restart: the definition, lifecycle status, and the owner
-- binding captured while the owner was live. Terminal output is never stored.
CREATE TABLE session_monitors (
  id TEXT PRIMARY KEY NOT NULL,
  session_id TEXT NOT NULL,
  workspace_id TEXT NOT NULL,
  profile_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('terminal', 'timer')),
  description TEXT NOT NULL,
  terminal_id TEXT NOT NULL DEFAULT '',
  until TEXT NOT NULL DEFAULT '',
  pattern TEXT NOT NULL DEFAULT '',
  timeout_ms INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL CHECK (status IN ('active', 'delivering', 'fired', 'canceled', 'undelivered', 'interrupted')),
  created_at TEXT NOT NULL,
  deadline_at TEXT NOT NULL,
  fired_at TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  exit_code INTEGER,
  owner_binding TEXT NOT NULL DEFAULT '',
  version INTEGER NOT NULL
);
CREATE INDEX idx_session_monitors_session ON session_monitors (session_id);
CREATE INDEX idx_session_monitors_status ON session_monitors (status, fired_at);
