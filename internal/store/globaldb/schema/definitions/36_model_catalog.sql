CREATE TABLE model_catalog_execution_contexts (
			context_id          TEXT NOT NULL CHECK (trim(context_id) <> ''),
			scope               TEXT NOT NULL CHECK (scope IN ('global', 'profile', 'workspace')),
			profile_id          TEXT NOT NULL DEFAULT '',
			workspace_id        TEXT NOT NULL DEFAULT '',
			command_fingerprint TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (context_id),
			UNIQUE (scope, profile_id, workspace_id, command_fingerprint),
			CHECK (
				(scope = 'global' AND profile_id = '' AND workspace_id = '' AND command_fingerprint = '') OR
				(scope = 'profile' AND trim(profile_id) <> '' AND workspace_id = '' AND trim(command_fingerprint) <> '') OR
				(scope = 'workspace' AND trim(profile_id) <> '' AND trim(workspace_id) <> '' AND trim(command_fingerprint) <> '')
			)
		);

CREATE TABLE model_catalog_reasoning_efforts (
			context_id  TEXT NOT NULL CHECK (trim(context_id) <> ''),
			source_id   TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			model_id    TEXT NOT NULL,
			effort      TEXT NOT NULL CHECK (trim(effort) <> ''),
			rank        INTEGER NOT NULL CHECK (rank >= 0),
			PRIMARY KEY (context_id, source_id, provider_id, model_id, effort),
			FOREIGN KEY (context_id, source_id, provider_id, model_id)
				REFERENCES model_catalog_rows(context_id, source_id, provider_id, model_id)
				ON DELETE CASCADE
		);

CREATE TABLE model_catalog_rows (
			context_id               TEXT NOT NULL CHECK (trim(context_id) <> ''),
			source_id                TEXT NOT NULL CHECK (trim(source_id) <> ''),
			provider_id              TEXT NOT NULL CHECK (trim(provider_id) <> ''),
			model_id                 TEXT NOT NULL CHECK (trim(model_id) <> ''),
			source_kind              TEXT NOT NULL CHECK (trim(source_kind) <> ''),
			priority                 INTEGER NOT NULL,
			available                INTEGER CHECK (available IN (0, 1) OR available IS NULL),
			stale                    INTEGER NOT NULL DEFAULT 0 CHECK (stale IN (0, 1)),
			refreshed_at             TEXT NOT NULL DEFAULT '',
			expires_at               TEXT NOT NULL DEFAULT '',
			display_name             TEXT NOT NULL DEFAULT '',
			context_window           INTEGER,
			max_input_tokens         INTEGER,
			max_output_tokens        INTEGER,
			supports_tools           INTEGER CHECK (supports_tools IN (0, 1) OR supports_tools IS NULL),
			supports_reasoning       INTEGER CHECK (supports_reasoning IN (0, 1) OR supports_reasoning IS NULL),
			default_reasoning_effort TEXT,
			cost_input_per_million   REAL,
			cost_output_per_million  REAL,
			cost_cache_read_per_million  REAL,
			cost_cache_write_per_million REAL,
			cost_reasoning_per_million   REAL,
			last_error               TEXT NOT NULL DEFAULT '', deprecated INTEGER NOT NULL DEFAULT 0 CHECK (deprecated IN (0, 1)), hidden INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)), featured INTEGER NOT NULL DEFAULT 0 CHECK (featured IN (0, 1)), release_date TEXT, explicitly_curated INTEGER NOT NULL DEFAULT 0 CHECK (explicitly_curated IN (0, 1)), deprecated_set INTEGER NOT NULL DEFAULT 0 CHECK (deprecated_set IN (0, 1)), hidden_set INTEGER NOT NULL DEFAULT 0 CHECK (hidden_set IN (0, 1)), featured_set INTEGER NOT NULL DEFAULT 0 CHECK (featured_set IN (0, 1)),
			PRIMARY KEY (context_id, source_id, provider_id, model_id),
			FOREIGN KEY (context_id, source_id, provider_id)
				REFERENCES model_catalog_sources(context_id, source_id, provider_id)
				ON DELETE CASCADE
		);

CREATE TABLE model_catalog_transport_bindings (
			context_id        TEXT NOT NULL CHECK (trim(context_id) <> ''),
			source_id          TEXT NOT NULL CHECK (trim(source_id) <> ''),
			provider_id        TEXT NOT NULL CHECK (trim(provider_id) <> ''),
			model_id           TEXT NOT NULL CHECK (trim(model_id) <> ''),
			transport_model_id TEXT NOT NULL CHECK (trim(transport_model_id) <> ''),
			label              TEXT NOT NULL DEFAULT '',
			reasoning_effort   TEXT,
			fast               INTEGER CHECK (fast IN (0, 1) OR fast IS NULL),
			thinking           INTEGER CHECK (thinking IN (0, 1) OR thinking IS NULL),
			rank               INTEGER NOT NULL CHECK (rank >= 0),
			PRIMARY KEY (context_id, source_id, provider_id, model_id, transport_model_id),
			FOREIGN KEY (context_id, source_id, provider_id, model_id)
				REFERENCES model_catalog_rows(context_id, source_id, provider_id, model_id)
				ON DELETE CASCADE
		);

CREATE INDEX idx_model_catalog_transport_bindings_row
			ON model_catalog_transport_bindings(context_id, source_id, provider_id, model_id, rank, transport_model_id);

CREATE TABLE model_catalog_options (
			context_id      TEXT NOT NULL CHECK (trim(context_id) <> ''),
			source_id        TEXT NOT NULL CHECK (trim(source_id) <> ''),
			provider_id      TEXT NOT NULL CHECK (trim(provider_id) <> ''),
			model_id         TEXT NOT NULL CHECK (trim(model_id) <> ''),
			option_id        TEXT NOT NULL CHECK (trim(option_id) <> ''),
			label            TEXT NOT NULL DEFAULT '',
			description      TEXT NOT NULL DEFAULT '',
			category         TEXT NOT NULL DEFAULT '',
			kind             TEXT NOT NULL CHECK (trim(kind) <> ''),
			current_value_id TEXT,
			current_bool     INTEGER CHECK (current_bool IN (0, 1) OR current_bool IS NULL),
			PRIMARY KEY (context_id, source_id, provider_id, model_id, option_id),
			FOREIGN KEY (context_id, source_id, provider_id, model_id)
				REFERENCES model_catalog_rows(context_id, source_id, provider_id, model_id)
				ON DELETE CASCADE,
			CHECK (current_value_id IS NULL OR trim(current_value_id) <> ''),
			CHECK (NOT (current_value_id IS NOT NULL AND current_bool IS NOT NULL))
		);

CREATE TABLE model_catalog_option_values (
			context_id  TEXT NOT NULL CHECK (trim(context_id) <> ''),
			source_id   TEXT NOT NULL CHECK (trim(source_id) <> ''),
			provider_id TEXT NOT NULL CHECK (trim(provider_id) <> ''),
			model_id    TEXT NOT NULL CHECK (trim(model_id) <> ''),
			option_id   TEXT NOT NULL CHECK (trim(option_id) <> ''),
			value_id    TEXT NOT NULL CHECK (trim(value_id) <> ''),
			label       TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			group_id    TEXT NOT NULL DEFAULT '',
			group_label TEXT NOT NULL DEFAULT '',
			rank        INTEGER NOT NULL CHECK (rank >= 0),
			PRIMARY KEY (context_id, source_id, provider_id, model_id, option_id, value_id),
			FOREIGN KEY (context_id, source_id, provider_id, model_id, option_id)
				REFERENCES model_catalog_options(context_id, source_id, provider_id, model_id, option_id)
				ON DELETE CASCADE
		);

CREATE INDEX idx_model_catalog_option_values_option
			ON model_catalog_option_values(context_id, source_id, provider_id, model_id, option_id, rank, value_id);

CREATE TABLE model_catalog_transport_binding_selections (
			context_id        TEXT NOT NULL CHECK (trim(context_id) <> ''),
			source_id          TEXT NOT NULL CHECK (trim(source_id) <> ''),
			provider_id        TEXT NOT NULL CHECK (trim(provider_id) <> ''),
			model_id           TEXT NOT NULL CHECK (trim(model_id) <> ''),
			transport_model_id TEXT NOT NULL CHECK (trim(transport_model_id) <> ''),
			option_id          TEXT NOT NULL CHECK (trim(option_id) <> ''),
			value_id           TEXT,
			bool_value         INTEGER CHECK (bool_value IN (0, 1) OR bool_value IS NULL),
			PRIMARY KEY (context_id, source_id, provider_id, model_id, transport_model_id, option_id),
			FOREIGN KEY (context_id, source_id, provider_id, model_id, transport_model_id)
				REFERENCES model_catalog_transport_bindings(context_id, source_id, provider_id, model_id, transport_model_id)
				ON DELETE CASCADE,
			FOREIGN KEY (context_id, source_id, provider_id, model_id, option_id)
				REFERENCES model_catalog_options(context_id, source_id, provider_id, model_id, option_id)
				ON DELETE CASCADE,
			FOREIGN KEY (context_id, source_id, provider_id, model_id, option_id, value_id)
				REFERENCES model_catalog_option_values(context_id, source_id, provider_id, model_id, option_id, value_id)
				ON DELETE CASCADE,
			CHECK (value_id IS NULL OR trim(value_id) <> ''),
			CHECK ((value_id IS NOT NULL AND bool_value IS NULL) OR (value_id IS NULL AND bool_value IS NOT NULL))
		);

CREATE INDEX idx_model_catalog_binding_selections_binding
			ON model_catalog_transport_binding_selections(
				context_id, source_id, provider_id, model_id, transport_model_id, option_id
			);

CREATE TABLE model_catalog_sources (
			context_id      TEXT NOT NULL CHECK (trim(context_id) <> ''),
			source_id       TEXT NOT NULL CHECK (trim(source_id) <> ''),
			provider_id     TEXT NOT NULL CHECK (trim(provider_id) <> ''),
			source_kind     TEXT NOT NULL CHECK (trim(source_kind) <> ''),
			priority        INTEGER NOT NULL,
			refresh_state   TEXT NOT NULL CHECK (trim(refresh_state) <> ''),
			last_refresh_at TEXT NOT NULL DEFAULT '',
			next_refresh_at TEXT NOT NULL DEFAULT '',
			last_success_at TEXT NOT NULL DEFAULT '',
			last_error      TEXT NOT NULL DEFAULT '',
			row_count       INTEGER NOT NULL DEFAULT 0 CHECK (row_count >= 0),
			stale           INTEGER NOT NULL DEFAULT 0 CHECK (stale IN (0, 1)),
			PRIMARY KEY (context_id, source_id, provider_id),
			FOREIGN KEY (context_id)
				REFERENCES model_catalog_execution_contexts(context_id)
				ON DELETE CASCADE
		);

CREATE INDEX idx_model_catalog_rows_provider_model
			ON model_catalog_rows(context_id, provider_id, model_id, priority DESC, refreshed_at DESC, source_id ASC);

CREATE INDEX idx_model_catalog_rows_source_provider
			ON model_catalog_rows(context_id, source_id, provider_id);

CREATE INDEX idx_model_catalog_sources_provider
			ON model_catalog_sources(context_id, provider_id, refresh_state, stale);
