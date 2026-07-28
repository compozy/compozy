CREATE TABLE config_apply_records (
			id                  TEXT PRIMARY KEY,
			desired_config_hash TEXT NOT NULL,
			active_config_hash  TEXT NOT NULL,
			generation          INTEGER NOT NULL CHECK (generation >= 0),
			actor               TEXT NOT NULL,
			diff_class          TEXT NOT NULL,
			status              TEXT NOT NULL CHECK (status IN ('pending_apply', 'applied', 'blocked', 'failed')),
			diagnostic_json     TEXT NOT NULL DEFAULT '',
			created_at          TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			applied_at          TEXT,
			updated_at          TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

CREATE INDEX idx_config_apply_records_active
			ON config_apply_records(active_config_hash, created_at DESC);

CREATE INDEX idx_config_apply_records_actor
			ON config_apply_records(actor, created_at DESC);

CREATE INDEX idx_config_apply_records_desired
			ON config_apply_records(desired_config_hash, created_at DESC);

CREATE INDEX idx_config_apply_records_generation
			ON config_apply_records(generation DESC, created_at DESC);

CREATE INDEX idx_config_apply_records_status
			ON config_apply_records(status, updated_at DESC);
