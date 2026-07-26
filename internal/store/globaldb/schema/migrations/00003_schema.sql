-- +goose Up
PRAGMA foreign_keys = off;

ALTER TABLE permission_log RENAME TO old_permission_log;

CREATE TABLE permission_log (
		id          TEXT PRIMARY KEY,
		session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		agent_name  TEXT NOT NULL,
		action      TEXT NOT NULL,
		resource    TEXT NOT NULL,
		decision    TEXT NOT NULL,
		policy_used TEXT NOT NULL,
		timestamp   TEXT NOT NULL
	);

INSERT INTO permission_log (
	id, session_id, agent_name, action, resource, decision, policy_used, timestamp
)
SELECT id, session_id, agent_name, action, resource, decision, policy_used, timestamp
FROM old_permission_log;

DROP TABLE old_permission_log;

CREATE INDEX idx_perm_session ON permission_log(session_id);

ALTER TABLE token_stats RENAME TO old_token_stats;

CREATE TABLE token_stats (
		id            TEXT PRIMARY KEY,
		session_id    TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		agent_name    TEXT NOT NULL,
		input_tokens  INTEGER,
		output_tokens INTEGER,
		total_tokens  INTEGER,
		total_cost    REAL,
		cost_currency TEXT,
		turn_count    INTEGER NOT NULL DEFAULT 0,
		updated_at    TEXT NOT NULL
	);

INSERT INTO token_stats (
	id, session_id, agent_name, input_tokens, output_tokens, total_tokens,
	total_cost, cost_currency, turn_count, updated_at
)
SELECT
	id, session_id, agent_name, input_tokens, output_tokens, total_tokens,
	total_cost, cost_currency, turn_count, updated_at
FROM old_token_stats;

DROP TABLE old_token_stats;

CREATE INDEX idx_token_stats_session ON token_stats(session_id);
CREATE UNIQUE INDEX idx_token_stats_session_agent
	ON token_stats(session_id, agent_name);

PRAGMA foreign_keys = on;
