-- +goose Up
CREATE TABLE new_automation_suggestions (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	source TEXT NOT NULL CHECK (source IN ('catalog', 'usage', 'integration')),
	dedup_key TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('pending', 'accepted', 'dismissed')),
	payload TEXT NOT NULL CHECK (json_valid(payload) AND json_type(payload) = 'object'),
	created_at TEXT NOT NULL,
	resolved_at TEXT,
	UNIQUE (workspace_id, dedup_key),
	CHECK (
		(status = 'pending' AND resolved_at IS NULL) OR
		(status IN ('accepted', 'dismissed') AND resolved_at IS NOT NULL)
	)
);
INSERT INTO new_automation_suggestions (
	id, workspace_id, source, dedup_key, status, payload, created_at, resolved_at
)
SELECT id, workspace_id, source, dedup_key, status, payload_json, created_at, resolved_at
FROM automation_suggestions;
DROP TABLE automation_suggestions;
ALTER TABLE new_automation_suggestions RENAME TO automation_suggestions;
CREATE UNIQUE INDEX automation_suggestions_workspace_id_dedup_key
	ON automation_suggestions(workspace_id, dedup_key);
CREATE INDEX idx_automation_suggestions_workspace_status
	ON automation_suggestions(workspace_id, status, created_at, id);

-- +goose Down
CREATE TABLE new_automation_suggestions (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	source TEXT NOT NULL CHECK (source IN ('catalog', 'usage', 'integration')),
	dedup_key TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('pending', 'accepted', 'dismissed')),
	payload_json TEXT NOT NULL CHECK (json_valid(payload_json) AND json_type(payload_json) = 'object'),
	created_at TEXT NOT NULL,
	resolved_at TEXT,
	UNIQUE (workspace_id, dedup_key),
	CHECK (
		(status = 'pending' AND resolved_at IS NULL) OR
		(status IN ('accepted', 'dismissed') AND resolved_at IS NOT NULL)
	)
);
INSERT INTO new_automation_suggestions (
	id, workspace_id, source, dedup_key, status, payload_json, created_at, resolved_at
)
SELECT id, workspace_id, source, dedup_key, status, payload, created_at, resolved_at
FROM automation_suggestions;
DROP TABLE automation_suggestions;
ALTER TABLE new_automation_suggestions RENAME TO automation_suggestions;
CREATE UNIQUE INDEX automation_suggestions_workspace_id_dedup_key
	ON automation_suggestions(workspace_id, dedup_key);
CREATE INDEX idx_automation_suggestions_workspace_status
	ON automation_suggestions(workspace_id, status, created_at, id);
