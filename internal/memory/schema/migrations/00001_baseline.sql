-- +goose Up

CREATE TABLE "memory_catalog_entries" (
		id           TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL DEFAULT '',
		scope        TEXT NOT NULL CHECK (scope IN ('global', 'workspace', 'agent')),
		agent_name   TEXT NOT NULL DEFAULT '',
		agent_tier   TEXT NOT NULL DEFAULT '' CHECK (agent_tier IN ('', 'workspace', 'global')),
		type         TEXT NOT NULL CHECK (type IN ('user', 'feedback', 'project', 'reference')),
		slug         TEXT NOT NULL,
		filename     TEXT NOT NULL,
		name         TEXT NOT NULL DEFAULT '',
		description  TEXT NOT NULL DEFAULT '',
		content      TEXT NOT NULL DEFAULT '',
		content_hash TEXT NOT NULL,
		injection    INTEGER NOT NULL DEFAULT 1,
		mtime_ms     INTEGER NOT NULL,
		indexed_at   INTEGER NOT NULL,
		updated_at   TEXT NOT NULL
	);

CREATE VIRTUAL TABLE memory_catalog_fts USING fts5(
		name,
		description,
		content,
		content='memory_catalog_entries',
		content_rowid='rowid',
		tokenize='porter unicode61'
	);

CREATE TABLE memory_catalog_state (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

CREATE TABLE memory_chunks (
		id           TEXT PRIMARY KEY,
		file_id      TEXT NOT NULL REFERENCES memory_catalog_entries(id) ON DELETE CASCADE,
		content      TEXT NOT NULL,
		content_hash TEXT NOT NULL,
		start_line   INTEGER NOT NULL,
		end_line     INTEGER NOT NULL,
		indexed_at   INTEGER NOT NULL
	);

CREATE VIRTUAL TABLE memory_chunks_fts USING fts5(
		content,
		content='memory_chunks',
		content_rowid='rowid',
		tokenize='unicode61'
	);

CREATE VIRTUAL TABLE memory_chunks_fts_trigram USING fts5(
		content,
		content='memory_chunks',
		content_rowid='rowid',
		tokenize='trigram'
	);

CREATE TABLE memory_consolidations (
		id             TEXT PRIMARY KEY,
		workspace_id   TEXT,
		scope          TEXT NOT NULL CHECK (scope IN ('global', 'workspace', 'agent')),
		agent_name     TEXT,
		agent_tier     TEXT CHECK (agent_tier IS NULL OR agent_tier IN ('workspace', 'global')),
		started_at     INTEGER NOT NULL,
		finished_at    INTEGER,
		status         TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'canceled')),
		input_count    INTEGER NOT NULL DEFAULT 0,
		promoted_count INTEGER NOT NULL DEFAULT 0,
		error          TEXT NOT NULL DEFAULT '',
		metadata       TEXT NOT NULL DEFAULT '{}'
	);

CREATE TABLE memory_decisions (
		id                TEXT PRIMARY KEY,
		candidate_hash    TEXT NOT NULL,
		idempotency_key   TEXT NOT NULL UNIQUE,
		frontmatter_hash  TEXT NOT NULL,
		workspace_id      TEXT,
		scope             TEXT NOT NULL CHECK (scope IN ('global', 'workspace', 'agent')),
		agent_name        TEXT,
		agent_tier        TEXT CHECK (agent_tier IS NULL OR agent_tier IN ('workspace', 'global')),
		op                TEXT NOT NULL CHECK (op IN ('noop', 'add', 'update', 'delete', 'reject')),
		targets           TEXT NOT NULL DEFAULT '[]',
		target_filename   TEXT NOT NULL,
		frontmatter       TEXT NOT NULL DEFAULT '{}',
		post_content      TEXT,
		post_content_hash TEXT,
		prior_content     TEXT,
		confidence        REAL NOT NULL,
		source            TEXT NOT NULL CHECK (source IN ('rule', 'llm')),
		rule_trace        TEXT NOT NULL,
		llm_trace         TEXT,
		reason            TEXT,
		prompt_version    TEXT,
		applied_at        INTEGER,
		decided_at        INTEGER NOT NULL
	);

CREATE TABLE memory_events (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		op           TEXT NOT NULL CHECK (op IN ('memory.write.committed', 'memory.write.rejected', 'memory.write.shadowed', 'memory.write.reindex', 'memory.write.reverted', 'memory.recall.executed', 'memory.recall.skipped', 'memory.recall.signal_dropped', 'memory.recall.signal_update_failed', 'memory.decisions.audit_summarized', 'memory.decisions.pruned', 'memory.dream.run.started', 'memory.dream.run.promoted', 'memory.dream.run.failed', 'memory.extractor.started', 'memory.extractor.completed', 'memory.extractor.failed', 'memory.extractor.coalesced', 'memory.extractor.dropped', 'memory.daily.rotated', 'memory.daily.archived', 'memory.daily.restored', 'memory.daily.purged', 'memory.daily.archive_purged', 'memory.provider.enabled', 'memory.provider.disabled', 'memory.provider.collision', 'memory.workspace.relocated', 'memory.workspace.recovered', 'memory.agent.purged', 'memory.migration.applied')),
		scope        TEXT CHECK (scope IN ('global', 'workspace', 'agent')),
		agent_name   TEXT,
		agent_tier   TEXT CHECK (agent_tier IS NULL OR agent_tier IN ('workspace', 'global')),
		workspace_id TEXT,
		session_id   TEXT,
		actor_kind   TEXT NOT NULL,
		decision_id  TEXT,
		target_id    TEXT,
		metadata     TEXT NOT NULL DEFAULT '{}',
		ts_ms        INTEGER NOT NULL
	);

CREATE TABLE memory_recall_signals (
		chunk_id              TEXT PRIMARY KEY REFERENCES memory_chunks(id) ON DELETE CASCADE,
		workspace_id          TEXT,
		recall_count          INTEGER NOT NULL DEFAULT 0,
		last_recalled_at      INTEGER,
		recall_score          REAL NOT NULL DEFAULT 0,
		freshness_started_at  INTEGER NOT NULL DEFAULT 0,
		promoted_at           INTEGER,
		promotion_run_id      TEXT,
		last_score_update_at  INTEGER NOT NULL DEFAULT 0,
		session_count         INTEGER NOT NULL DEFAULT 0,
		last_session_id       TEXT,
		already_surfaced_json TEXT NOT NULL DEFAULT '[]',
		updated_at            INTEGER NOT NULL
	);

CREATE INDEX idx_chunks_file ON memory_chunks(file_id);

CREATE INDEX idx_consolidations_status
		ON memory_consolidations(status, started_at);

CREATE INDEX idx_consolidations_workspace
		ON memory_consolidations(workspace_id, started_at);

CREATE INDEX idx_decisions_op ON memory_decisions(op, decided_at);

CREATE INDEX idx_decisions_unapplied
		ON memory_decisions(applied_at) WHERE applied_at IS NULL;

CREATE INDEX idx_decisions_workspace ON memory_decisions(workspace_id, decided_at);

CREATE INDEX idx_events_op ON memory_events(op, ts_ms);

CREATE INDEX idx_events_session ON memory_events(session_id, ts_ms);

CREATE INDEX idx_events_workspace ON memory_events(workspace_id, ts_ms);

CREATE INDEX idx_memory_catalog_scope
		ON memory_catalog_entries(scope, agent_name, agent_tier, type);

CREATE INDEX idx_memory_catalog_updated_at
		ON memory_catalog_entries(updated_at);

CREATE INDEX idx_memory_catalog_workspace
		ON memory_catalog_entries(workspace_id);

CREATE INDEX idx_recall_signals_last_recalled
		ON memory_recall_signals(last_recalled_at);

CREATE INDEX idx_recall_signals_workspace
		ON memory_recall_signals(workspace_id, updated_at);

CREATE INDEX idx_signals_recent
		ON memory_recall_signals(last_recalled_at);

CREATE INDEX idx_signals_unpromoted
		ON memory_recall_signals(promoted_at, recall_score) WHERE promoted_at IS NULL;

CREATE UNIQUE INDEX uq_memory_catalog_new_scope_slug
		 ON "memory_catalog_entries"(workspace_id, scope, agent_name, agent_tier, type, slug);

CREATE UNIQUE INDEX uq_memory_catalog_scope_slug
		ON memory_catalog_entries(workspace_id, scope, agent_name, agent_tier, type, slug);

-- +goose StatementBegin
CREATE TRIGGER memory_catalog_entries_ad AFTER DELETE ON memory_catalog_entries BEGIN
		INSERT INTO memory_catalog_fts(memory_catalog_fts, rowid, name, description, content)
		VALUES ('delete', old.rowid, old.name, old.description, old.content);
	END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER memory_catalog_entries_ai AFTER INSERT ON memory_catalog_entries BEGIN
		INSERT INTO memory_catalog_fts(rowid, name, description, content)
		VALUES (new.rowid, new.name, new.description, new.content);
	END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER memory_catalog_entries_au AFTER UPDATE ON memory_catalog_entries BEGIN
		INSERT INTO memory_catalog_fts(memory_catalog_fts, rowid, name, description, content)
		VALUES ('delete', old.rowid, old.name, old.description, old.content);
		INSERT INTO memory_catalog_fts(rowid, name, description, content)
		VALUES (new.rowid, new.name, new.description, new.content);
	END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER memory_chunks_ad AFTER DELETE ON memory_chunks BEGIN
		INSERT INTO memory_chunks_fts(memory_chunks_fts, rowid, content)
		VALUES ('delete', old.rowid, old.content);
		INSERT INTO memory_chunks_fts_trigram(memory_chunks_fts_trigram, rowid, content)
		VALUES ('delete', old.rowid, old.content);
	END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER memory_chunks_ai AFTER INSERT ON memory_chunks BEGIN
		INSERT INTO memory_chunks_fts(rowid, content) VALUES (new.rowid, new.content);
		INSERT INTO memory_chunks_fts_trigram(rowid, content) VALUES (new.rowid, new.content);
	END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER memory_chunks_au AFTER UPDATE ON memory_chunks BEGIN
		INSERT INTO memory_chunks_fts(memory_chunks_fts, rowid, content)
		VALUES ('delete', old.rowid, old.content);
		INSERT INTO memory_chunks_fts_trigram(memory_chunks_fts_trigram, rowid, content)
		VALUES ('delete', old.rowid, old.content);
		INSERT INTO memory_chunks_fts(rowid, content) VALUES (new.rowid, new.content);
		INSERT INTO memory_chunks_fts_trigram(rowid, content) VALUES (new.rowid, new.content);
	END;
-- +goose StatementEnd
