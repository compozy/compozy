CREATE TABLE automation_job_catalog_entries (
			job_id                   TEXT PRIMARY KEY REFERENCES automation_jobs(id) ON DELETE CASCADE,
			scope                    TEXT NOT NULL,
			workspace_id             TEXT NOT NULL DEFAULT '',
			source                   TEXT NOT NULL,
			source_rank              INTEGER NOT NULL,
			name                     TEXT NOT NULL,
			loop_name                TEXT NOT NULL DEFAULT '',
			enabled                  BOOLEAN NOT NULL,
			search_name              TEXT NOT NULL,
			search_agent_name        TEXT NOT NULL,
			search_prompt            TEXT NOT NULL,
			search_scope             TEXT NOT NULL,
			search_source            TEXT NOT NULL,
			search_schedule_mode     TEXT NOT NULL,
			search_schedule_expr     TEXT NOT NULL,
			search_schedule_interval TEXT NOT NULL,
			search_schedule_time     TEXT NOT NULL
		);

CREATE TABLE automation_job_overlays (
		job_id            TEXT PRIMARY KEY,
		enabled_override  BOOLEAN NOT NULL,
		updated_at        TEXT NOT NULL
	);

CREATE TABLE automation_jobs (
		id           TEXT PRIMARY KEY,
		scope        TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
		name         TEXT NOT NULL,
		agent_name   TEXT NOT NULL,
		workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		prompt       TEXT NOT NULL,
		schedule     TEXT,
		task         TEXT,
		enabled      BOOLEAN NOT NULL DEFAULT 1,
		retry        TEXT NOT NULL,
		fire_limit   TEXT NOT NULL,
		source       TEXT NOT NULL DEFAULT 'dynamic',
		target_kind  TEXT NOT NULL DEFAULT 'agent' CHECK (target_kind IN ('agent', 'loop')),
		loop_workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		loop_name    TEXT,
		loop_inputs  TEXT,
		loop_input_mapping TEXT,
		loop_network_participation TEXT CHECK (
			loop_network_participation IS NULL OR json_valid(loop_network_participation)
		),
		created_at   TEXT NOT NULL,
		updated_at   TEXT NOT NULL,
		CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		)
	);

CREATE TABLE automation_suggestions (
		id           TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		source       TEXT NOT NULL CHECK (source IN ('catalog', 'usage', 'integration')),
		dedup_key    TEXT NOT NULL,
		status       TEXT NOT NULL CHECK (status IN ('pending', 'accepted', 'dismissed')),
		payload      TEXT NOT NULL CHECK (json_valid(payload) AND json_type(payload) = 'object'),
		created_at   TEXT NOT NULL,
		resolved_at  TEXT,
		CHECK (
			(status = 'pending' AND resolved_at IS NULL) OR
			(status IN ('accepted', 'dismissed') AND resolved_at IS NOT NULL)
		)
	);

CREATE TABLE automation_runs (
		id         TEXT PRIMARY KEY,
		job_id     TEXT,
		trigger_id TEXT,
		session_id TEXT,
		task_id    TEXT,
		task_run_id TEXT,
		status     TEXT NOT NULL,
		attempt    INTEGER NOT NULL DEFAULT 1,
		started_at TEXT,
		ended_at   TEXT,
		error      TEXT,
		loop_run_id TEXT REFERENCES loop_runs(id) ON DELETE SET NULL
	, fire_id TEXT, scheduled_at TEXT, delivery_error TEXT, delivery_error_at TEXT,
	  network_participation TEXT CHECK (
		network_participation IS NULL OR json_valid(network_participation)
	  ), metadata_json TEXT NOT NULL DEFAULT '{}');

CREATE TABLE "automation_scheduler_state" (
			job_id                       TEXT PRIMARY KEY,
			next_run_at                  TEXT,
			last_run_at                  TEXT,
			last_scheduled_at            TEXT,
			last_fire_id                 TEXT NOT NULL DEFAULT '',
			schedule_hash                TEXT NOT NULL DEFAULT '',
			catch_up_policy              TEXT NOT NULL DEFAULT 'skip_missed'
				CHECK (catch_up_policy IN ('skip_missed', 'coalesce', 'replay', 'run_once_on_catchup')),
			misfire_grace_seconds        INTEGER NOT NULL DEFAULT 0
				CHECK (misfire_grace_seconds >= 0),
			consecutive_resume_failures  INTEGER NOT NULL DEFAULT 0
				CHECK (consecutive_resume_failures >= 0),
			last_misfire_at              TEXT,
			misfire_count                INTEGER NOT NULL DEFAULT 0
				CHECK (misfire_count >= 0),
			updated_at                   TEXT NOT NULL
		);

CREATE TABLE automation_trigger_catalog_entries (
			trigger_id           TEXT PRIMARY KEY REFERENCES automation_triggers(id) ON DELETE CASCADE,
			scope                TEXT NOT NULL,
			workspace_id         TEXT NOT NULL DEFAULT '',
			event                TEXT NOT NULL,
			source               TEXT NOT NULL,
			source_rank          INTEGER NOT NULL,
			name                 TEXT NOT NULL,
			loop_name            TEXT NOT NULL DEFAULT '',
			enabled              BOOLEAN NOT NULL,
			search_name          TEXT NOT NULL,
			search_agent_name    TEXT NOT NULL,
			search_prompt        TEXT NOT NULL,
			search_scope         TEXT NOT NULL,
			search_source        TEXT NOT NULL,
			search_event         TEXT NOT NULL,
			search_endpoint_slug TEXT NOT NULL,
			search_webhook_id    TEXT NOT NULL
		);

CREATE TABLE automation_trigger_catalog_filter_terms (
			trigger_id TEXT NOT NULL REFERENCES automation_triggers(id) ON DELETE CASCADE,
			value      TEXT NOT NULL,
			PRIMARY KEY (trigger_id, value)
		);

CREATE TABLE automation_trigger_overlays (
		trigger_id        TEXT PRIMARY KEY,
		enabled_override  BOOLEAN NOT NULL,
		updated_at        TEXT NOT NULL
	);

CREATE TABLE automation_triggers (
		id            TEXT PRIMARY KEY,
		scope         TEXT NOT NULL CHECK (scope IN ('global', 'workspace')),
		name          TEXT NOT NULL,
		agent_name    TEXT NOT NULL,
		workspace_id  TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		prompt        TEXT NOT NULL,
		event         TEXT NOT NULL,
		filter        TEXT,
		enabled       BOOLEAN NOT NULL DEFAULT 1,
		retry         TEXT NOT NULL,
		fire_limit    TEXT NOT NULL,
		source        TEXT NOT NULL DEFAULT 'dynamic',
		webhook_id    TEXT,
		endpoint_slug TEXT,
		webhook_secret_ref TEXT,
		target_kind   TEXT NOT NULL DEFAULT 'agent' CHECK (target_kind IN ('agent', 'loop')),
		loop_workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
		loop_name     TEXT,
		loop_inputs   TEXT,
		loop_input_mapping TEXT,
		loop_network_participation TEXT CHECK (
			loop_network_participation IS NULL OR json_valid(loop_network_participation)
		),
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL,
		CHECK (
			(scope = 'global' AND workspace_id IS NULL) OR
			(scope = 'workspace' AND workspace_id IS NOT NULL)
		)
	);

CREATE TABLE automation_watch_events (
	seq          INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id       TEXT NOT NULL,
	job_id       TEXT NOT NULL DEFAULT '',
	trigger_id   TEXT NOT NULL DEFAULT '',
	session_id   TEXT NOT NULL DEFAULT '',
	status       TEXT NOT NULL CHECK (status IN ('completed', 'failed')),
	attempt      INTEGER NOT NULL,
	started_at   TEXT,
	ended_at     TEXT,
	error        TEXT NOT NULL DEFAULT '',
	agent_name   TEXT NOT NULL DEFAULT '',
	workspace_id TEXT NOT NULL DEFAULT '',
	retry_json   TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_automation_job_catalog_order
			ON automation_job_catalog_entries(source_rank, name, job_id);

CREATE INDEX idx_automation_job_catalog_workspace_order
			ON automation_job_catalog_entries(workspace_id, source_rank, name, job_id);

CREATE INDEX idx_automation_jobs_enabled ON automation_jobs(enabled);

CREATE INDEX idx_automation_jobs_loop_target
			ON automation_jobs(loop_name, loop_workspace_id) WHERE target_kind = 'loop';

CREATE INDEX idx_automation_runs_job ON automation_runs(job_id);

CREATE INDEX idx_automation_runs_loop_run ON automation_runs(loop_run_id);

CREATE INDEX idx_automation_runs_started ON automation_runs(started_at);

CREATE INDEX idx_automation_runs_status ON automation_runs(status);

CREATE INDEX idx_automation_runs_trigger ON automation_runs(trigger_id);

CREATE INDEX idx_automation_scheduler_misfire
			ON automation_scheduler_state(last_misfire_at);

CREATE INDEX idx_automation_scheduler_next_run
			ON automation_scheduler_state(next_run_at);

CREATE INDEX idx_automation_suggestions_workspace_status
			ON automation_suggestions(workspace_id, status, created_at, id);

CREATE UNIQUE INDEX automation_suggestions_workspace_id_dedup_key
			ON automation_suggestions(workspace_id, dedup_key);

CREATE INDEX idx_automation_trigger_catalog_order
			ON automation_trigger_catalog_entries(source_rank, name, trigger_id);

CREATE INDEX idx_automation_trigger_catalog_workspace_order
			ON automation_trigger_catalog_entries(workspace_id, source_rank, name, trigger_id);

CREATE INDEX idx_automation_triggers_enabled ON automation_triggers(enabled);

CREATE INDEX idx_automation_triggers_event ON automation_triggers(event);

CREATE INDEX idx_automation_triggers_loop_target
			ON automation_triggers(loop_name, loop_workspace_id) WHERE target_kind = 'loop';

CREATE INDEX idx_automation_watch_events_workspace_status_seq
			ON automation_watch_events(workspace_id, status, seq);

CREATE UNIQUE INDEX uq_automation_jobs_global_name ON automation_jobs(name) WHERE scope = 'global';

CREATE UNIQUE INDEX uq_automation_jobs_workspace_name ON automation_jobs(workspace_id, name) WHERE scope = 'workspace';

CREATE UNIQUE INDEX uq_automation_runs_fire_id
			ON automation_runs(fire_id) WHERE fire_id IS NOT NULL;

CREATE UNIQUE INDEX uq_automation_triggers_global_name ON automation_triggers(name) WHERE scope = 'global';

CREATE UNIQUE INDEX uq_automation_triggers_webhook_id ON automation_triggers(webhook_id) WHERE webhook_id IS NOT NULL;

CREATE UNIQUE INDEX uq_automation_triggers_workspace_name ON automation_triggers(workspace_id, name) WHERE scope = 'workspace';

CREATE TRIGGER automation_watch_events_after_insert
AFTER INSERT ON automation_runs
WHEN NEW.status IN ('completed', 'failed')
BEGIN
	INSERT INTO automation_watch_events (
		run_id, job_id, trigger_id, session_id, status, attempt,
		started_at, ended_at, error, agent_name, workspace_id, retry_json
	) VALUES (
		NEW.id,
		COALESCE(NEW.job_id, ''),
		COALESCE(NEW.trigger_id, ''),
		COALESCE(NEW.session_id, ''),
		NEW.status,
		NEW.attempt,
		NEW.started_at,
		NEW.ended_at,
		COALESCE(NEW.error, ''),
		COALESCE(
			(SELECT agent_name FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT agent_name FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		),
		COALESCE(
			NULLIF((SELECT workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			NULLIF((SELECT workspace_id FROM loop_runs WHERE id = NEW.loop_run_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			''
		),
		COALESCE(
			(SELECT retry FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT retry FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		)
	);
END;

CREATE TRIGGER automation_watch_events_after_terminal_update
AFTER UPDATE OF status ON automation_runs
WHEN NEW.status IN ('completed', 'failed') AND OLD.status NOT IN ('completed', 'failed')
BEGIN
	INSERT INTO automation_watch_events (
		run_id, job_id, trigger_id, session_id, status, attempt,
		started_at, ended_at, error, agent_name, workspace_id, retry_json
	) VALUES (
		NEW.id,
		COALESCE(NEW.job_id, ''),
		COALESCE(NEW.trigger_id, ''),
		COALESCE(NEW.session_id, ''),
		NEW.status,
		NEW.attempt,
		NEW.started_at,
		NEW.ended_at,
		COALESCE(NEW.error, ''),
		COALESCE(
			(SELECT agent_name FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT agent_name FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		),
		COALESCE(
			NULLIF((SELECT workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			NULLIF((SELECT workspace_id FROM loop_runs WHERE id = NEW.loop_run_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_jobs WHERE id = NEW.job_id), ''),
			NULLIF((SELECT loop_workspace_id FROM automation_triggers WHERE id = NEW.trigger_id), ''),
			''
		),
		COALESCE(
			(SELECT retry FROM automation_jobs WHERE id = NEW.job_id),
			(SELECT retry FROM automation_triggers WHERE id = NEW.trigger_id),
			''
		)
	);
END;
