-- +goose Up

CREATE TABLE events (
		id         TEXT PRIMARY KEY,
		sequence   INTEGER NOT NULL,
		turn_id    TEXT NOT NULL,
		type       TEXT NOT NULL,
		agent_name TEXT NOT NULL,
		content    TEXT NOT NULL,
		timestamp  TEXT NOT NULL
	, transcript_entry_key TEXT NOT NULL DEFAULT '');

CREATE TABLE hook_runs (
		id             TEXT PRIMARY KEY,
		hook_name      TEXT NOT NULL,
		event          TEXT NOT NULL,
		source         TEXT NOT NULL,
		mode           TEXT NOT NULL,
		duration_ns    INTEGER NOT NULL,
		outcome        TEXT NOT NULL,
		dispatch_depth INTEGER NOT NULL,
		patch_applied  TEXT,
		error          TEXT,
		required       INTEGER NOT NULL DEFAULT 0,
		recorded_at    TEXT NOT NULL
	);

CREATE TABLE token_usage (
		turn_id            TEXT PRIMARY KEY,
		input_tokens       INTEGER,
		output_tokens      INTEGER,
		total_tokens       INTEGER,
		thought_tokens     INTEGER,
		cache_read_tokens  INTEGER,
		cache_write_tokens INTEGER,
		context_used       INTEGER,
		context_size       INTEGER,
		cost_amount        REAL,
		cost_currency      TEXT,
		timestamp          TEXT NOT NULL
	);

CREATE TABLE transcript_entries (
		entry_key TEXT PRIMARY KEY,
		kind TEXT NOT NULL,
		logical_id TEXT NOT NULL,
		turn_id TEXT NOT NULL,
		base_message_id TEXT NOT NULL,
		message_id TEXT,
		start_sequence INTEGER NOT NULL UNIQUE,
		updated_sequence INTEGER NOT NULL,
		complete INTEGER NOT NULL,
		message_json TEXT,
		event_type TEXT NOT NULL DEFAULT '',
		marker_json TEXT,
		CHECK(updated_sequence >= start_sequence)
	);

CREATE TABLE transcript_projection_state (
		singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
		projection_version INTEGER NOT NULL,
		generation INTEGER NOT NULL,
		active_entry_key TEXT
	);

CREATE TABLE transcript_tool_routes (
		tool_key TEXT PRIMARY KEY,
		entry_key TEXT NOT NULL
	);

CREATE UNIQUE INDEX idx_events_sequence ON events(sequence);

CREATE INDEX idx_events_timestamp ON events(timestamp);

CREATE INDEX idx_events_transcript_entry_sequence
		ON events(transcript_entry_key, sequence);

CREATE INDEX idx_events_turn ON events(turn_id);

CREATE INDEX idx_events_type ON events(type);

CREATE INDEX idx_hook_runs_event ON hook_runs(event);

CREATE INDEX idx_hook_runs_recorded_at ON hook_runs(recorded_at);

CREATE UNIQUE INDEX idx_transcript_entries_message_id
		ON transcript_entries(message_id) WHERE message_id IS NOT NULL;

CREATE INDEX idx_transcript_entries_turn
		ON transcript_entries(kind, turn_id, start_sequence);

CREATE INDEX idx_transcript_entries_updated
		ON transcript_entries(updated_sequence, start_sequence);

CREATE INDEX idx_transcript_entries_visible_start
		ON transcript_entries(start_sequence) WHERE message_json IS NOT NULL;

CREATE INDEX idx_usage_timestamp ON token_usage(timestamp);

