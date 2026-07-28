CREATE TABLE network_audit_log (
			id         TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			direction  TEXT NOT NULL,
			kind       TEXT NOT NULL,
		channel    TEXT NOT NULL,
		surface    TEXT,
		thread_id  TEXT,
		direct_id  TEXT,
		work_id    TEXT,
		peer_from  TEXT NOT NULL,
		peer_to    TEXT,
		message_id TEXT NOT NULL,
		reason     TEXT,
		size       INTEGER NOT NULL,
		timestamp  TEXT NOT NULL
	);

CREATE TABLE network_channel_kind_counts (
		workspace_id TEXT NOT NULL,
		channel      TEXT NOT NULL,
		kind         TEXT NOT NULL,
		message_count INTEGER NOT NULL CHECK (message_count >= 0),
		PRIMARY KEY (workspace_id, channel, kind)
	);

CREATE TABLE network_channel_participants (
		workspace_id TEXT NOT NULL,
		channel      TEXT NOT NULL,
		session_id   TEXT NOT NULL,
		PRIMARY KEY (workspace_id, channel, session_id),
		FOREIGN KEY (workspace_id, session_id)
			REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
	);

CREATE TABLE network_channel_stats (
		workspace_id                 TEXT NOT NULL,
		channel                      TEXT NOT NULL,
		message_count                INTEGER NOT NULL DEFAULT 0 CHECK (message_count >= 0),
		presence_count               INTEGER NOT NULL DEFAULT 0 CHECK (presence_count >= 0),
		historical_participant_count INTEGER NOT NULL DEFAULT 0 CHECK (historical_participant_count >= 0),
		last_activity_at             TEXT,
		last_presence_at             TEXT,
		last_message_id              TEXT NOT NULL DEFAULT '',
		last_message_preview         TEXT NOT NULL DEFAULT '', last_activity_sequence INTEGER NOT NULL DEFAULT 0, last_presence_sequence INTEGER NOT NULL DEFAULT 0, last_message_sequence INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (workspace_id, channel)
	);

CREATE TABLE network_channels (
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			channel      TEXT NOT NULL,
			purpose      TEXT NOT NULL,
			created_by   TEXT NOT NULL DEFAULT '',
			created_at   TEXT NOT NULL,
			updated_at   TEXT NOT NULL, fanout_policy TEXT NOT NULL DEFAULT 'capability_match' CHECK (
					fanout_policy IN ('capability_match', 'coordinator', 'all_members')
				), coordinator_peer_id TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (workspace_id, channel)
		);

CREATE TABLE network_direct_rooms (
		workspace_id         TEXT NOT NULL,
		channel              TEXT NOT NULL,
		direct_id            TEXT NOT NULL,
		session_a            TEXT NOT NULL,
		session_b            TEXT NOT NULL,
		opened_at            TEXT NOT NULL,
		last_activity_at     TEXT NOT NULL,
		message_count        INTEGER NOT NULL DEFAULT 0 CHECK (message_count >= 0),
		open_work_count      INTEGER NOT NULL DEFAULT 0 CHECK (open_work_count >= 0),
		last_message_preview TEXT NOT NULL DEFAULT '', opened_sequence INTEGER NOT NULL DEFAULT 0, last_activity_sequence INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (workspace_id, channel, direct_id),
		UNIQUE (workspace_id, channel, session_a, session_b),
		CHECK (session_a < session_b),
		FOREIGN KEY (workspace_id, session_a)
			REFERENCES sessions(workspace_id, id) ON DELETE CASCADE,
		FOREIGN KEY (workspace_id, session_b)
			REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
	);

CREATE TABLE network_subscriptions (
			workspace_id          TEXT NOT NULL,
			channel               TEXT NOT NULL,
			thread_id             TEXT NOT NULL DEFAULT '',
			session_id            TEXT NOT NULL,
			mode                  TEXT NOT NULL CHECK (mode IN ('mute', 'full')),
			created_at            TEXT NOT NULL,
			updated_at            TEXT NOT NULL,
			PRIMARY KEY (workspace_id, channel, thread_id, session_id),
			FOREIGN KEY (workspace_id, channel)
				REFERENCES network_channels(workspace_id, channel)
				ON DELETE CASCADE,
			FOREIGN KEY (workspace_id, session_id)
				REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
		);

CREATE TABLE network_task_thread_origins (
			task_id                 TEXT PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
			workspace_id            TEXT NOT NULL,
			channel                 TEXT NOT NULL,
			thread_id               TEXT NOT NULL,
			origin_message_id       TEXT NOT NULL,
			digest                  TEXT NOT NULL,
			source_message_ids_json TEXT NOT NULL DEFAULT '[]',
			created_at              TEXT NOT NULL,
			updated_at              TEXT NOT NULL,
			FOREIGN KEY (workspace_id, channel, thread_id)
				REFERENCES network_threads(workspace_id, channel, thread_id)
				ON DELETE CASCADE
		);

CREATE TABLE network_task_status_projections (
		event_id             TEXT NOT NULL REFERENCES task_events(id) ON DELETE CASCADE,
		recipient_session_id TEXT NOT NULL,
		workspace_id         TEXT NOT NULL,
		channel              TEXT NOT NULL,
		thread_id            TEXT NOT NULL,
		task_id              TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		run_id               TEXT NOT NULL DEFAULT '',
		event_type           TEXT NOT NULL,
		projection_json      TEXT NOT NULL CHECK (json_valid(projection_json)),
		projected_at         TEXT NOT NULL,
		PRIMARY KEY (event_id, recipient_session_id),
		FOREIGN KEY (workspace_id, channel, thread_id)
			REFERENCES network_threads(workspace_id, channel, thread_id)
			ON DELETE CASCADE,
		FOREIGN KEY (workspace_id, recipient_session_id)
			REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
	);

CREATE TABLE network_thread_participants (
		workspace_id     TEXT NOT NULL,
		channel          TEXT NOT NULL,
		thread_id        TEXT NOT NULL,
		session_id       TEXT NOT NULL,
		first_message_id TEXT NOT NULL,
		first_seen_at    TEXT NOT NULL,
		last_seen_at     TEXT NOT NULL,
		PRIMARY KEY (workspace_id, channel, thread_id, session_id),
		FOREIGN KEY (workspace_id, channel, thread_id)
			REFERENCES network_threads(workspace_id, channel, thread_id)
			ON DELETE CASCADE,
		FOREIGN KEY (workspace_id, session_id)
			REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
	);

CREATE TABLE network_thread_session_token_stats (
			workspace_id             TEXT NOT NULL,
			channel                  TEXT NOT NULL,
			thread_id                TEXT NOT NULL,
			session_id               TEXT NOT NULL,
			delivered_count          INTEGER NOT NULL DEFAULT 0 CHECK (delivered_count >= 0),
			prompt_size_bytes        INTEGER NOT NULL DEFAULT 0 CHECK (prompt_size_bytes >= 0),
			estimated_prompt_tokens  INTEGER NOT NULL DEFAULT 0 CHECK (estimated_prompt_tokens >= 0),
			first_delivered_at       TEXT NOT NULL,
			last_delivered_at        TEXT NOT NULL,
			updated_at              TEXT NOT NULL,
			PRIMARY KEY (workspace_id, channel, thread_id, session_id),
			FOREIGN KEY (workspace_id, channel, thread_id)
				REFERENCES network_threads(workspace_id, channel, thread_id)
				ON DELETE CASCADE,
			FOREIGN KEY (workspace_id, session_id)
				REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
		);

CREATE TABLE network_threads (
		workspace_id         TEXT NOT NULL,
		channel              TEXT NOT NULL,
		thread_id            TEXT NOT NULL,
		root_message_id      TEXT NOT NULL,
		title                TEXT NOT NULL DEFAULT '',
		opened_by_peer_id    TEXT NOT NULL DEFAULT '',
		opened_session_id    TEXT NOT NULL DEFAULT '',
		opened_at            TEXT NOT NULL,
		last_activity_at     TEXT NOT NULL,
		message_count        INTEGER NOT NULL DEFAULT 0 CHECK (message_count >= 0),
		participant_count    INTEGER NOT NULL DEFAULT 0 CHECK (participant_count >= 0),
		open_work_count      INTEGER NOT NULL DEFAULT 0 CHECK (open_work_count >= 0),
		last_message_preview TEXT NOT NULL DEFAULT '', opened_sequence INTEGER NOT NULL DEFAULT 0, last_activity_sequence INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (workspace_id, channel, thread_id)
	);

CREATE TABLE network_timeline_log (
	sequence        INTEGER PRIMARY KEY AUTOINCREMENT,
	message_id      TEXT NOT NULL,
	session_id      TEXT,
	workspace_id    TEXT NOT NULL,
	channel         TEXT NOT NULL,
	surface         TEXT CHECK (surface IN ('thread', 'direct') OR surface IS NULL),
	thread_id       TEXT,
	direct_id       TEXT,
	direction       TEXT NOT NULL,
	peer_from       TEXT NOT NULL,
	peer_to         TEXT,
	kind            TEXT NOT NULL,
	work_id         TEXT,
	reply_to        TEXT,
	trace_id        TEXT,
	causation_id    TEXT,
	intent          TEXT,
	text            TEXT,
	preview_text    TEXT NOT NULL DEFAULT '',
	body_json       TEXT NOT NULL,
	timestamp       TEXT NOT NULL,
	ext_json        TEXT NOT NULL DEFAULT '{}',
	mentions_json   TEXT NOT NULL DEFAULT '[]',
	work_opened     INTEGER NOT NULL DEFAULT 0 CHECK (work_opened IN (0, 1)),
	work_transitioned INTEGER NOT NULL DEFAULT 0 CHECK (work_transitioned IN (0, 1)),
	work_state      TEXT NOT NULL DEFAULT '' CHECK (
		work_state IN ('', 'submitted', 'working', 'needs_input', 'completed', 'failed', 'canceled')
	),
	UNIQUE (workspace_id, message_id),
	CHECK (
		(surface IS NULL AND thread_id IS NULL AND direct_id IS NULL AND work_id IS NULL
			AND kind IN ('greet', 'whois'))
		OR (surface = 'thread' AND thread_id IS NOT NULL AND direct_id IS NULL)
		OR (surface = 'direct' AND direct_id IS NOT NULL AND thread_id IS NULL)
	),
	CHECK (kind IN ('greet', 'whois', 'say', 'capability', 'receipt', 'trace'))
);

CREATE TABLE network_work (
		work_id           TEXT NOT NULL,
		workspace_id      TEXT NOT NULL,
		channel           TEXT NOT NULL,
		surface           TEXT NOT NULL CHECK (surface IN ('thread', 'direct')),
		thread_id         TEXT,
		direct_id         TEXT,
		opened_by_session_id TEXT NOT NULL,
		target_session_id    TEXT,
		state             TEXT NOT NULL CHECK (
			state IN ('submitted', 'working', 'needs_input', 'completed', 'failed', 'canceled')
		),
		opened_at         TEXT NOT NULL,
		last_activity_at  TEXT NOT NULL,
		terminal_at       TEXT,
		CHECK (
			(surface = 'thread' AND thread_id IS NOT NULL AND direct_id IS NULL)
			OR (surface = 'direct' AND direct_id IS NOT NULL AND thread_id IS NULL)
		),
		PRIMARY KEY (workspace_id, work_id),
		FOREIGN KEY (workspace_id, channel, thread_id)
			REFERENCES network_threads(workspace_id, channel, thread_id)
			ON DELETE RESTRICT,
		FOREIGN KEY (workspace_id, channel, direct_id)
			REFERENCES network_direct_rooms(workspace_id, channel, direct_id)
			ON DELETE RESTRICT,
		FOREIGN KEY (workspace_id, opened_by_session_id)
			REFERENCES sessions(workspace_id, id) ON DELETE CASCADE,
		FOREIGN KEY (workspace_id, target_session_id)
			REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
	);

CREATE TABLE workspace_network_coordination (
	workspace_id TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
	enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
	revision INTEGER NOT NULL CHECK (revision >= 1),
	updated_at TEXT NOT NULL,
	updated_by TEXT NOT NULL CHECK (length(trim(updated_by)) > 0)
);

CREATE TABLE task_network_coordination (
	task_id TEXT NOT NULL PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
	revision INTEGER NOT NULL CHECK (revision >= 1),
	updated_at TEXT NOT NULL,
	updated_by TEXT NOT NULL CHECK (length(trim(updated_by)) > 0)
);

CREATE INDEX idx_task_network_coordination_workspace
	ON task_network_coordination(workspace_id, task_id);

CREATE TABLE network_coordination_invitations (
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	scope_kind TEXT NOT NULL CHECK (scope_kind IN ('workspace', 'task')),
	scope_id TEXT NOT NULL CHECK (length(trim(scope_id)) > 0),
	dismissed_at TEXT NOT NULL,
	dismissed_by TEXT NOT NULL CHECK (length(trim(dismissed_by)) > 0),
	PRIMARY KEY (workspace_id, scope_kind, scope_id),
	CHECK (scope_kind <> 'workspace' OR scope_id = workspace_id)
);

CREATE TABLE network_availability (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
	epoch INTEGER NOT NULL CHECK (epoch >= 1),
	updated_at TEXT NOT NULL,
	updated_by TEXT NOT NULL CHECK (length(trim(updated_by)) > 0)
);

CREATE TABLE network_message_dispositions (
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	message_id TEXT NOT NULL,
	recipient_session_id TEXT NOT NULL,
	decision TEXT NOT NULL,
	decided_at TEXT NOT NULL,
	acceptance_seq INTEGER NOT NULL CHECK (acceptance_seq >= 1),
	PRIMARY KEY (workspace_id, message_id, recipient_session_id),
	FOREIGN KEY (workspace_id, recipient_session_id)
		REFERENCES sessions(workspace_id, id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, message_id)
		REFERENCES network_timeline_log(workspace_id, message_id) ON DELETE CASCADE
);

CREATE TABLE network_live_wakes (
	wake_id TEXT NOT NULL,
	task_run_id TEXT NOT NULL UNIQUE REFERENCES task_runs(id) ON DELETE CASCADE,
	owner_key TEXT NOT NULL CHECK (length(trim(owner_key)) > 0),
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	channel TEXT NOT NULL,
	root_id TEXT NOT NULL,
	depth INTEGER NOT NULL CHECK (depth >= 0),
	state TEXT NOT NULL CHECK (state IN ('open', 'succeeded', 'failed', 'canceled', 'deadline_exceeded')),
	coalesce_until TEXT NOT NULL,
	reserved_wall_ms INTEGER NOT NULL CHECK (reserved_wall_ms > 0),
	actual_wall_ms INTEGER CHECK (actual_wall_ms IS NULL OR actual_wall_ms >= 0),
	reserved_at TEXT NOT NULL,
	settled_at TEXT,
	input_tokens INTEGER CHECK (input_tokens IS NULL OR input_tokens >= 0),
	output_tokens INTEGER CHECK (output_tokens IS NULL OR output_tokens >= 0),
	usage_state TEXT NOT NULL DEFAULT '' CHECK (usage_state IN ('', 'actual', 'usage_unavailable')),
	reason TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (workspace_id, wake_id),
	UNIQUE (workspace_id, wake_id, owner_key),
	FOREIGN KEY (workspace_id, task_run_id)
		REFERENCES task_runs(workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE network_wake_sources (
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	owner_key TEXT NOT NULL,
	envelope_id TEXT NOT NULL,
	wake_id TEXT NOT NULL,
	PRIMARY KEY (workspace_id, owner_key, envelope_id),
	FOREIGN KEY (workspace_id, wake_id, owner_key)
		REFERENCES network_live_wakes(workspace_id, wake_id, owner_key) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, envelope_id)
		REFERENCES network_timeline_log(workspace_id, message_id) ON DELETE CASCADE
);

CREATE TABLE network_participation_budgets (
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	owner_key TEXT NOT NULL CHECK (length(trim(owner_key)) > 0),
	wakes_used INTEGER NOT NULL DEFAULT 0 CHECK (wakes_used >= 0),
	wall_ms_used INTEGER NOT NULL DEFAULT 0 CHECK (wall_ms_used >= 0),
	input_tokens_used INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens_used >= 0),
	output_tokens_used INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens_used >= 0),
	exhausted_reason TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, owner_key)
);

CREATE TABLE network_wake_events (
	sequence INTEGER PRIMARY KEY AUTOINCREMENT,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	wake_id TEXT NOT NULL,
	task_run_id TEXT NOT NULL,
	owner_key TEXT NOT NULL CHECK (length(trim(owner_key)) > 0),
	target_session_id TEXT NOT NULL,
	event_type TEXT NOT NULL CHECK (event_type IN ('admitted', 'claimed', 'heartbeat', 'released', 'settled')),
	state TEXT NOT NULL,
	claim_token_hash TEXT NOT NULL DEFAULT '',
	usage_state TEXT NOT NULL DEFAULT '' CHECK (usage_state IN ('', 'actual', 'usage_unavailable')),
	actual_wall_ms INTEGER CHECK (actual_wall_ms IS NULL OR actual_wall_ms >= 0),
	input_tokens INTEGER CHECK (input_tokens IS NULL OR input_tokens >= 0),
	output_tokens INTEGER CHECK (output_tokens IS NULL OR output_tokens >= 0),
	reason TEXT NOT NULL DEFAULT '',
	actor_kind TEXT NOT NULL,
	actor_ref TEXT NOT NULL,
	timestamp TEXT NOT NULL,
	FOREIGN KEY (workspace_id, wake_id, owner_key)
		REFERENCES network_live_wakes(workspace_id, wake_id, owner_key) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, task_run_id)
		REFERENCES task_runs(workspace_id, id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, target_session_id)
		REFERENCES sessions(workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_net_audit_conversation
			ON network_audit_log(workspace_id, channel, surface, thread_id, direct_id, timestamp);

CREATE INDEX idx_net_audit_ts ON network_audit_log(timestamp);

CREATE INDEX idx_net_audit_work
			ON network_audit_log(workspace_id, work_id, timestamp)
			WHERE work_id IS NOT NULL;

CREATE INDEX idx_net_audit_workspace_session ON network_audit_log(workspace_id, session_id);

CREATE INDEX idx_net_timeline_direct_sequence
		ON network_timeline_log(workspace_id, channel, direct_id, sequence)
		WHERE surface = 'direct';

CREATE INDEX idx_net_timeline_kind_sequence
		ON network_timeline_log(workspace_id, kind, sequence);

CREATE INDEX idx_net_timeline_presence_sequence
		ON network_timeline_log(workspace_id, channel, sequence)
		WHERE surface IS NULL;

CREATE INDEX idx_net_timeline_thread_sequence
		ON network_timeline_log(workspace_id, channel, thread_id, sequence)
		WHERE surface = 'thread';

CREATE INDEX idx_net_timeline_work_sequence
		ON network_timeline_log(workspace_id, work_id, sequence)
		WHERE work_id IS NOT NULL;

CREATE INDEX idx_network_channel_participants_session
		ON network_channel_participants(workspace_id, session_id, channel);

CREATE INDEX idx_network_channel_stats_activity
		ON network_channel_stats(workspace_id, last_activity_sequence DESC, channel);

CREATE INDEX idx_network_channels_updated_at ON network_channels(updated_at);

CREATE INDEX idx_network_channels_workspace ON network_channels(workspace_id);

CREATE INDEX idx_network_channels_workspace_updated_at
			ON network_channels(workspace_id, updated_at DESC, channel ASC);

CREATE INDEX idx_network_direct_rooms_activity
		ON network_direct_rooms(workspace_id, channel, last_activity_sequence DESC, direct_id);

CREATE INDEX idx_network_direct_rooms_created
		ON network_direct_rooms(workspace_id, channel, opened_at, direct_id);

CREATE INDEX idx_network_direct_rooms_open_work
		ON network_direct_rooms(workspace_id, channel, open_work_count, last_activity_sequence DESC, direct_id);

CREATE INDEX idx_network_direct_rooms_session_a
		ON network_direct_rooms(workspace_id, channel, session_a, last_activity_sequence DESC);

CREATE INDEX idx_network_direct_rooms_session_b
		ON network_direct_rooms(workspace_id, channel, session_b, last_activity_sequence DESC);

CREATE INDEX idx_network_subscriptions_session
			ON network_subscriptions(workspace_id, session_id, channel, thread_id);

CREATE INDEX idx_network_task_thread_origins_thread
			ON network_task_thread_origins(workspace_id, channel, thread_id, created_at DESC);

CREATE INDEX idx_network_task_status_projections_recipient
			ON network_task_status_projections(workspace_id, recipient_session_id, projected_at DESC, event_id);

CREATE INDEX idx_network_task_status_projections_task
			ON network_task_status_projections(workspace_id, task_id, projected_at DESC, event_id);

CREATE INDEX idx_network_thread_participants_session
		ON network_thread_participants(workspace_id, session_id, last_seen_at DESC);

CREATE INDEX idx_network_thread_session_token_stats_session
			ON network_thread_session_token_stats(workspace_id, channel, session_id, last_delivered_at DESC);

CREATE INDEX idx_network_threads_activity
		ON network_threads(workspace_id, channel, last_activity_sequence DESC, thread_id);

CREATE INDEX idx_network_threads_created
		ON network_threads(workspace_id, channel, opened_sequence, thread_id);

CREATE INDEX idx_network_threads_open_work
		ON network_threads(workspace_id, channel, open_work_count, last_activity_sequence DESC, thread_id);

CREATE INDEX idx_network_threads_title
		ON network_threads(workspace_id, channel, title COLLATE NOCASE, thread_id);

CREATE INDEX idx_network_work_conversation
		ON network_work(workspace_id, channel, surface, thread_id, direct_id, last_activity_at DESC);

CREATE INDEX idx_network_work_state
		ON network_work(workspace_id, state, last_activity_at DESC);

CREATE UNIQUE INDEX uq_network_live_wakes_open_owner
	ON network_live_wakes(workspace_id, owner_key)
	WHERE state = 'open';

CREATE INDEX idx_network_live_wakes_workspace_channel
	ON network_live_wakes(workspace_id, channel, reserved_at DESC, wake_id);

CREATE INDEX idx_network_wake_events_wake_sequence
	ON network_wake_events(workspace_id, wake_id, sequence);
