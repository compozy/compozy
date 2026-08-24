CREATE TABLE notification_cursors (
			scope_kind        TEXT NOT NULL CHECK (scope_kind IN ('global', 'workspace')),
			profile_id        TEXT NOT NULL REFERENCES profiles(id),
			workspace_id      TEXT NOT NULL DEFAULT '' CHECK (
				(scope_kind = 'global' AND workspace_id = '') OR
(scope_kind = 'workspace' AND workspace_id <> '')
			),
			consumer_id       TEXT NOT NULL CHECK (consumer_id <> ''),
			stream_name       TEXT NOT NULL CHECK (stream_name <> ''),
			subject_id        TEXT NOT NULL DEFAULT '',
			last_sequence     INTEGER NOT NULL DEFAULT 0 CHECK (last_sequence >= 0),
			last_delivery_id  TEXT NOT NULL DEFAULT '',
			last_delivered_at TEXT,
			last_error        TEXT NOT NULL DEFAULT '',
			updated_at        TEXT NOT NULL,
			PRIMARY KEY (scope_kind, profile_id, workspace_id, consumer_id, stream_name, subject_id)
		);

CREATE TABLE notification_presets (
			name                     TEXT PRIMARY KEY CHECK (trim(name) <> ''),
			events                   TEXT NOT NULL CHECK (json_valid(events)),
			targets                  TEXT NOT NULL CHECK (json_valid(targets)),
			filter                   TEXT NOT NULL DEFAULT '',
			built_in                 BOOLEAN NOT NULL DEFAULT 0 CHECK (built_in IN (0, 1)),
			default_version          TEXT NOT NULL DEFAULT '',
			default_hash             TEXT NOT NULL DEFAULT '',
			user_modified            BOOLEAN NOT NULL DEFAULT 0 CHECK (user_modified IN (0, 1)),
			default_update_available BOOLEAN NOT NULL DEFAULT 0 CHECK (default_update_available IN (0, 1)),
			created_at               TEXT NOT NULL,
			updated_at               TEXT NOT NULL
		);

CREATE INDEX idx_notification_presets_builtin
			ON notification_presets(built_in, name);

CREATE INDEX notification_cursors_stream_sequence_idx
			ON notification_cursors(scope_kind, profile_id, workspace_id, stream_name, last_sequence DESC)
			WHERE last_sequence > 0;
