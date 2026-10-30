-- Observations outlive session deletion. Exact OS identities, never bare PIDs, are authoritative.
CREATE TABLE session_process_observations (
  pid INTEGER NOT NULL CHECK (pid > 1),
  started_at TEXT NOT NULL,
  kernel_token TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL,
  workspace_id TEXT NOT NULL,
  profile_id TEXT NOT NULL,
  session_name TEXT NOT NULL,
  pgid INTEGER NOT NULL,
  first_seen TEXT NOT NULL,
  group_observed_at TEXT NOT NULL,
  attribution TEXT NOT NULL CHECK (attribution IN ('env', 'lineage')),
  PRIMARY KEY (pid, started_at)
);
CREATE INDEX idx_session_process_observations_owner ON session_process_observations (profile_id, workspace_id, session_id);
