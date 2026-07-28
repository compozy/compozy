CREATE TABLE model_catalog_reasoning_efforts (
			source_id   TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			model_id    TEXT NOT NULL,
			effort      TEXT NOT NULL CHECK (trim(effort) <> ''),
			rank        INTEGER NOT NULL CHECK (rank >= 0),
			PRIMARY KEY (source_id, provider_id, model_id, effort),
			FOREIGN KEY (source_id, provider_id, model_id)
				REFERENCES model_catalog_rows(source_id, provider_id, model_id)
				ON DELETE CASCADE
		);

CREATE TABLE model_catalog_rows (
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
			PRIMARY KEY (source_id, provider_id, model_id),
			FOREIGN KEY (source_id, provider_id)
				REFERENCES model_catalog_sources(source_id, provider_id)
				ON DELETE CASCADE
		);

CREATE TABLE model_catalog_sources (
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
			PRIMARY KEY (source_id, provider_id)
		);

CREATE INDEX idx_model_catalog_rows_provider_model
			ON model_catalog_rows(provider_id, model_id, priority DESC, refreshed_at DESC, source_id ASC);

CREATE INDEX idx_model_catalog_rows_source_provider
			ON model_catalog_rows(source_id, provider_id);

CREATE INDEX idx_model_catalog_sources_provider
			ON model_catalog_sources(provider_id, refresh_state, stale);
