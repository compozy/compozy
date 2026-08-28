CREATE TABLE terminal_recordings (
	id TEXT NOT NULL PRIMARY KEY,
	terminal_id TEXT NOT NULL,
	profile_id TEXT NOT NULL,
	digest TEXT NOT NULL,
	path TEXT NOT NULL,
	started_at INTEGER NOT NULL,
	stopped_at INTEGER,
	bytes INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);

CREATE INDEX idx_terminal_recordings_terminal_started
	ON terminal_recordings (terminal_id, started_at DESC, id DESC);
CREATE INDEX idx_terminal_recordings_profile_started
	ON terminal_recordings (profile_id, started_at DESC, id DESC);
CREATE INDEX idx_terminal_recordings_expires
	ON terminal_recordings (expires_at, id);
CREATE INDEX idx_terminal_recordings_path
	ON terminal_recordings (path, id);

-- Profiles live in globaldb. Availability is enforced at terminal admission;
-- this local trigger enforces only immutable ownership on historical rows.
CREATE TRIGGER terminal_recordings_profile_owner_immutable
BEFORE UPDATE OF profile_id ON terminal_recordings
WHEN NEW.profile_id <> OLD.profile_id
BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;

CREATE TABLE terminal_commands (
	id TEXT NOT NULL PRIMARY KEY,
	terminal_id TEXT,
	profile_id TEXT NOT NULL,
	actor_kind TEXT NOT NULL,
	actor_id TEXT NOT NULL,
	session_id TEXT,
	run_id TEXT,
	command TEXT NOT NULL,
	argv_digest TEXT,
	cwd TEXT NOT NULL,
	started_at INTEGER NOT NULL,
	duration_ms INTEGER,
	exit_code INTEGER,
	exit_signal TEXT,
	exit_cause TEXT NOT NULL,
	detected_by TEXT NOT NULL,
	approval TEXT NOT NULL,
	output_bytes INTEGER NOT NULL,
	truncated INTEGER NOT NULL,
	recording_id TEXT,
	FOREIGN KEY (recording_id) REFERENCES terminal_recordings (id)
);

CREATE INDEX idx_terminal_commands_terminal_started
	ON terminal_commands (terminal_id, started_at DESC, id DESC);
CREATE INDEX idx_terminal_commands_profile_started
	ON terminal_commands (profile_id, started_at DESC, id DESC);
CREATE INDEX idx_terminal_commands_started
	ON terminal_commands (started_at DESC, id DESC);
CREATE INDEX idx_terminal_commands_recording
	ON terminal_commands (recording_id);

CREATE TRIGGER terminal_commands_profile_owner_immutable
BEFORE UPDATE OF profile_id ON terminal_commands
WHEN NEW.profile_id <> OLD.profile_id
BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;

CREATE TABLE terminal_artifacts (
	id TEXT NOT NULL PRIMARY KEY,
	terminal_id TEXT,
	command_id TEXT NOT NULL,
	profile_id TEXT NOT NULL,
	digest TEXT NOT NULL,
	path TEXT NOT NULL,
	bytes INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	FOREIGN KEY (command_id) REFERENCES terminal_commands (id)
);

CREATE INDEX idx_terminal_artifacts_command
	ON terminal_artifacts (command_id, id);
CREATE INDEX idx_terminal_artifacts_profile
	ON terminal_artifacts (profile_id, id);
CREATE INDEX idx_terminal_artifacts_expires
	ON terminal_artifacts (expires_at, id);
CREATE INDEX idx_terminal_artifacts_path
	ON terminal_artifacts (path, id);

CREATE TRIGGER terminal_artifacts_profile_owner_immutable
BEFORE UPDATE OF profile_id ON terminal_artifacts
WHEN NEW.profile_id <> OLD.profile_id
BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
