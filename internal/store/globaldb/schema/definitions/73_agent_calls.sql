CREATE TABLE contract_schemas (
	digest TEXT PRIMARY KEY CHECK (digest GLOB 'sha256:[0-9a-f]*' AND length(digest) = 71),
	schema TEXT NOT NULL CHECK (json_valid(schema)),
	created_at TEXT NOT NULL
);

CREATE TABLE payload_blobs (
	workspace_id TEXT NOT NULL DEFAULT '',
	ref TEXT NOT NULL CHECK (ref GLOB 'sha256:[0-9a-f]*' AND length(ref) = 71),
	bytes BLOB NOT NULL,
	byte_size INTEGER NOT NULL CHECK (byte_size >= 0),
	created_at TEXT NOT NULL,
	last_used_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, ref),
	CHECK (byte_size = length(bytes))
);

CREATE TABLE calls (
	call_id TEXT PRIMARY KEY CHECK (call_id LIKE 'call_%'),
	profile_id TEXT NOT NULL REFERENCES profiles(id),
	scope TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
	workspace_id TEXT NOT NULL DEFAULT '',
	caller_kind TEXT NOT NULL CHECK (
		caller_kind IN ('session', 'task_run', 'loop_run', 'automation_run')
	),
	caller_id TEXT NOT NULL CHECK (trim(caller_id) <> ''),
	actor_kind TEXT NOT NULL CHECK (
		actor_kind IN ('human', 'agent_session', 'automation', 'extension', 'network_peer', 'daemon')
	),
	actor_id TEXT NOT NULL CHECK (trim(actor_id) <> ''),
	activation_run_id TEXT REFERENCES task_runs(id) ON DELETE SET NULL,
	parent_session_id TEXT REFERENCES sessions(id) ON DELETE SET NULL,
	agent_name TEXT,
	child_session_id TEXT REFERENCES sessions(id) ON DELETE SET NULL,
	governed_root_id TEXT NOT NULL CHECK (trim(governed_root_id) <> ''),
	depth INTEGER NOT NULL CHECK (depth >= 0),
	state TEXT NOT NULL CHECK (
		state IN (
			'queued', 'running', 'completed', 'invalid-result', 'completed-without-result',
			'failed', 'canceled', 'timeout', 'expired'
		)
	),
	verdict TEXT CHECK (verdict IS NULL OR verdict IN ('returned', 'extracted', 'repaired')),
	expect_digest TEXT REFERENCES contract_schemas(digest),
	prompt_ref TEXT NOT NULL,
	result_ref TEXT,
	result_bytes INTEGER CHECK (result_bytes IS NULL OR result_bytes >= 0),
	result_budget_bytes INTEGER NOT NULL CHECK (result_budget_bytes > 0),
	result_overflow TEXT NOT NULL CHECK (result_overflow IN ('store', 'reject')),
	strict INTEGER NOT NULL DEFAULT 0 CHECK (strict IN (0, 1)),
	idle_ttl_seconds INTEGER NOT NULL CHECK (idle_ttl_seconds > 0),
	runtime_provider TEXT NOT NULL DEFAULT '',
	runtime_model TEXT NOT NULL DEFAULT '',
	runtime_reasoning_effort TEXT NOT NULL DEFAULT '',
	runtime_speed TEXT NOT NULL DEFAULT '',
	failure_code TEXT,
	failure_detail TEXT CHECK (failure_detail IS NULL OR length(CAST(failure_detail AS BLOB)) <= 2048),
	repair_attempts INTEGER NOT NULL DEFAULT 0 CHECK (repair_attempts IN (0, 1)),
	first_issue_text TEXT NOT NULL DEFAULT '' CHECK (length(CAST(first_issue_text AS BLOB)) <= 4096),
	second_issue_text TEXT NOT NULL DEFAULT '' CHECK (length(CAST(second_issue_text AS BLOB)) <= 4096),
	final_prose_preview TEXT NOT NULL DEFAULT '' CHECK (length(CAST(final_prose_preview AS BLOB)) <= 4096),
	superseded_ref TEXT,
	idempotency_key TEXT,
	request_digest TEXT NOT NULL CHECK (trim(request_digest) <> ''),
	batch_id TEXT,
	deadline_at TEXT,
	created_at TEXT NOT NULL,
	started_at TEXT,
	settled_at TEXT,
	updated_at TEXT NOT NULL,
	CHECK ((scope = 'global' AND workspace_id = '') OR (scope = 'workspace' AND workspace_id <> '')),
	CHECK ((agent_name IS NULL) <> (child_session_id IS NULL) OR state <> 'queued'),
	CHECK ((state = 'completed') = (result_ref IS NOT NULL)),
	CHECK ((result_ref IS NULL AND result_bytes IS NULL) OR (result_ref IS NOT NULL AND result_bytes IS NOT NULL)),
	FOREIGN KEY (workspace_id, prompt_ref) REFERENCES payload_blobs(workspace_id, ref),
	FOREIGN KEY (workspace_id, result_ref) REFERENCES payload_blobs(workspace_id, ref),
	FOREIGN KEY (workspace_id, superseded_ref) REFERENCES payload_blobs(workspace_id, ref)
);

CREATE UNIQUE INDEX uq_calls_idempotency
	ON calls(profile_id, scope, workspace_id, caller_kind, caller_id, idempotency_key)
	WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_calls_owner_state
	ON calls(profile_id, scope, workspace_id, state, created_at DESC, call_id DESC);
CREATE INDEX idx_calls_caller_state ON calls(caller_kind, caller_id, state, created_at DESC);
CREATE INDEX idx_calls_child_state ON calls(child_session_id, state, created_at DESC);
CREATE INDEX idx_calls_root_state ON calls(governed_root_id, state);
CREATE INDEX idx_calls_deadline ON calls(deadline_at, call_id)
	WHERE deadline_at IS NOT NULL AND state IN ('queued', 'running');
CREATE INDEX idx_calls_activation_run ON calls(activation_run_id)
	WHERE activation_run_id IS NOT NULL;

CREATE TABLE call_permission_atoms (
	call_id TEXT NOT NULL REFERENCES calls(call_id) ON DELETE CASCADE,
	atom TEXT NOT NULL CHECK (trim(atom) <> ''),
	PRIMARY KEY (call_id, atom)
);

CREATE TABLE call_activation_runs (
	run_id TEXT PRIMARY KEY REFERENCES task_runs(id) ON DELETE CASCADE,
	call_id TEXT NOT NULL UNIQUE REFERENCES calls(call_id) ON DELETE CASCADE,
	workspace_id TEXT NOT NULL DEFAULT '',
	governed_root_id TEXT NOT NULL CHECK (trim(governed_root_id) <> ''),
	activation_kind TEXT NOT NULL CHECK (activation_kind IN ('spawn', 'revive')),
	parent_session_id TEXT REFERENCES sessions(id) ON DELETE CASCADE,
	target_session_id TEXT REFERENCES sessions(id) ON DELETE CASCADE,
	agent_name TEXT,
	depth INTEGER NOT NULL CHECK (depth >= 0),
	idle_ttl_seconds INTEGER NOT NULL CHECK (idle_ttl_seconds > 0),
	runtime_provider TEXT NOT NULL DEFAULT '',
	runtime_model TEXT NOT NULL DEFAULT '',
	runtime_reasoning_effort TEXT NOT NULL DEFAULT '',
	runtime_speed TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	CHECK (
		(activation_kind = 'spawn' AND agent_name IS NOT NULL AND target_session_id IS NULL) OR
		(activation_kind = 'revive' AND agent_name IS NULL AND target_session_id IS NOT NULL)
	)
);

CREATE INDEX idx_call_activation_runs_root ON call_activation_runs(governed_root_id, run_id);

CREATE TABLE operator_caller_sessions (
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	scope TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
	workspace_id TEXT NOT NULL DEFAULT '',
	session_id TEXT NOT NULL UNIQUE REFERENCES sessions(id) ON DELETE CASCADE,
	created_at TEXT NOT NULL,
	PRIMARY KEY (profile_id, scope, workspace_id),
	CHECK ((scope = 'global' AND workspace_id = '') OR (scope = 'workspace' AND workspace_id <> ''))
);

CREATE TABLE call_messages (
	message_id TEXT PRIMARY KEY CHECK (message_id LIKE 'msg_%'),
	profile_id TEXT NOT NULL REFERENCES profiles(id),
	scope TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
	workspace_id TEXT NOT NULL DEFAULT '',
	from_kind TEXT NOT NULL CHECK (from_kind IN ('session', 'operator')),
	from_id TEXT NOT NULL CHECK (trim(from_id) <> ''),
	to_session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	call_id TEXT REFERENCES calls(call_id) ON DELETE SET NULL,
	body TEXT NOT NULL,
	dedup_hash TEXT NOT NULL CHECK (trim(dedup_hash) <> ''),
	created_at TEXT NOT NULL,
	CHECK ((scope = 'global' AND workspace_id = '') OR (scope = 'workspace' AND workspace_id <> ''))
);

CREATE INDEX idx_call_messages_owner_time
	ON call_messages(profile_id, scope, workspace_id, created_at DESC, message_id DESC);
CREATE INDEX idx_call_messages_recipient_time ON call_messages(to_session_id, created_at, message_id);
CREATE INDEX idx_call_messages_sender_dedup ON call_messages(from_kind, from_id, dedup_hash, created_at DESC);

CREATE TABLE call_deliveries (
	delivery_id TEXT PRIMARY KEY CHECK (delivery_id LIKE 'delivery_%'),
	kind TEXT NOT NULL CHECK (kind IN ('completion', 'follow-up', 'message', 'repair')),
	subject_id TEXT NOT NULL CHECK (trim(subject_id) <> ''),
	recipient_session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	owner_key TEXT NOT NULL CHECK (trim(owner_key) <> ''),
	wake_event_id TEXT NOT NULL UNIQUE CHECK (trim(wake_event_id) <> ''),
	state TEXT NOT NULL CHECK (state IN ('pending', 'attention', 'injected', 'woken', 'failed')),
	reason TEXT NOT NULL DEFAULT '',
	attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	delivered_at TEXT,
	UNIQUE (kind, subject_id, recipient_session_id)
);

CREATE INDEX idx_call_deliveries_pending ON call_deliveries(state, created_at, delivery_id);
CREATE INDEX idx_call_deliveries_recipient ON call_deliveries(recipient_session_id, state, created_at);

CREATE TABLE call_publications (
	call_id TEXT NOT NULL REFERENCES calls(call_id) ON DELETE CASCADE,
	channel TEXT NOT NULL CHECK (trim(channel) <> ''),
	thread_id TEXT NOT NULL DEFAULT '',
	network_message_id TEXT NOT NULL CHECK (trim(network_message_id) <> ''),
	created_at TEXT NOT NULL,
	PRIMARY KEY (call_id, channel, thread_id)
);
