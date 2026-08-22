CREATE TABLE profiles (
	id TEXT PRIMARY KEY CHECK (length(id) = 26),
	name TEXT NOT NULL UNIQUE CHECK (trim(name) <> ''),
	color TEXT NOT NULL CHECK (trim(color) <> ''),
	icon TEXT,
	emoji TEXT,
	state TEXT NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'archived')),
	created_at TEXT NOT NULL,
	archived_at TEXT,
	CHECK ((icon IS NULL) <> (emoji IS NULL)),
	CHECK ((state = 'active' AND archived_at IS NULL) OR (state = 'archived' AND archived_at IS NOT NULL))
);

CREATE TABLE profile_selections (
	lens TEXT NOT NULL CHECK (lens IN ('workspace', 'global')),
	workspace_id TEXT NOT NULL DEFAULT '',
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (lens, workspace_id),
	CHECK ((lens = 'global' AND workspace_id = '') OR (lens = 'workspace' AND workspace_id <> ''))
);

CREATE TABLE profile_lifecycle_ops (
	id TEXT PRIMARY KEY CHECK (id LIKE 'op_%'),
	kind TEXT NOT NULL CHECK (kind IN ('create', 'rename', 'archive', 'unarchive', 'delete')),
	profile_id TEXT NOT NULL,
	old_name TEXT,
	new_name TEXT,
	plan_revision TEXT NOT NULL CHECK (trim(plan_revision) <> ''),
	status TEXT NOT NULL CHECK (status IN ('applied', 'finalizing', 'done', 'failed')),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	completed_at TEXT,
	error_code TEXT,
	error_message TEXT
);

CREATE INDEX idx_profile_lifecycle_ops_profile_status
	ON profile_lifecycle_ops (profile_id, status, updated_at DESC);

CREATE TABLE profile_lifecycle_op_steps (
	op_id TEXT NOT NULL REFERENCES profile_lifecycle_ops(id) ON DELETE CASCADE,
	seq INTEGER NOT NULL CHECK (seq >= 0),
	action TEXT NOT NULL CHECK (trim(action) <> ''),
	path_old TEXT,
	path_new TEXT,
	status TEXT NOT NULL CHECK (status IN ('pending', 'done', 'failed')),
	updated_at TEXT NOT NULL,
	error_message TEXT,
	PRIMARY KEY (op_id, seq)
);

CREATE TABLE profile_lifecycle_op_seed (
	op_id TEXT PRIMARY KEY REFERENCES profile_lifecycle_ops(id) ON DELETE CASCADE,
	color TEXT NOT NULL CHECK (trim(color) <> ''),
	icon TEXT,
	emoji TEXT,
	default_agent TEXT,
	default_provider TEXT,
	default_sandbox TEXT,
	declaration_digest TEXT NOT NULL CHECK (trim(declaration_digest) <> ''),
	CHECK ((icon IS NULL) <> (emoji IS NULL))
);

CREATE TABLE profile_lifecycle_op_credential_asks (
	op_id TEXT NOT NULL REFERENCES profile_lifecycle_ops(id) ON DELETE CASCADE,
	provider TEXT NOT NULL CHECK (trim(provider) <> ''),
	slot TEXT NOT NULL CHECK (trim(slot) <> ''),
	PRIMARY KEY (op_id, provider, slot)
);

CREATE TABLE profile_credential_requirements (
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	provider TEXT NOT NULL CHECK (trim(provider) <> ''),
	slot TEXT NOT NULL CHECK (trim(slot) <> ''),
	source_extension TEXT NOT NULL CHECK (trim(source_extension) <> ''),
	declaration_digest TEXT NOT NULL CHECK (trim(declaration_digest) <> ''),
	created_at TEXT NOT NULL,
	PRIMARY KEY (profile_id, provider, slot)
);

CREATE TABLE notification_delivery_permits (
	scope_kind TEXT NOT NULL CHECK (scope_kind IN ('global', 'workspace')),
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	workspace_id TEXT NOT NULL DEFAULT '',
	consumer_id TEXT NOT NULL CHECK (consumer_id <> ''),
	stream_name TEXT NOT NULL CHECK (stream_name <> ''),
	subject_id TEXT NOT NULL DEFAULT '',
	delivery_id TEXT NOT NULL CHECK (delivery_id <> ''),
	acquired_at TEXT NOT NULL,
	PRIMARY KEY (scope_kind, profile_id, workspace_id, consumer_id, stream_name, subject_id, delivery_id),
	CHECK ((scope_kind = 'global' AND workspace_id = '') OR (scope_kind = 'workspace' AND workspace_id <> ''))
);

CREATE TABLE extension_profile_enablement (
	extension_name TEXT NOT NULL,
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
	PRIMARY KEY (extension_name, profile_id)
);

CREATE TABLE extension_profile_markers (
	extension_name TEXT NOT NULL REFERENCES extensions(name) ON DELETE CASCADE,
	profile_name TEXT NOT NULL CHECK (trim(profile_name) <> ''),
	created_profile_id TEXT NOT NULL CHECK (trim(created_profile_id) <> ''),
	created_at TEXT NOT NULL,
	PRIMARY KEY (extension_name, profile_name)
);

CREATE TABLE notification_preset_enablement (
	preset_name TEXT NOT NULL REFERENCES notification_presets(name) ON DELETE CASCADE,
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
	PRIMARY KEY (preset_name, profile_id)
);

CREATE TABLE attention_workspace_mutes (
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	PRIMARY KEY (profile_id, workspace_id)
);
