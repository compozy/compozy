CREATE TABLE app_tokens (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL CHECK (length(trim(name)) > 0),
	token_hash TEXT NOT NULL CHECK (length(token_hash) = 64),
	revoke_epoch INTEGER NOT NULL DEFAULT 0 CHECK (revoke_epoch >= 0),
	created_at TEXT NOT NULL,
	last_seen_at TEXT,
	revoked_at TEXT
);

CREATE UNIQUE INDEX app_tokens_token_hash ON app_tokens(token_hash);

CREATE TABLE app_token_origins (
	app_token_id TEXT NOT NULL REFERENCES app_tokens(id) ON DELETE CASCADE,
	origin TEXT NOT NULL CHECK (length(origin) BETWEEN 8 AND 2048),
	created_at TEXT NOT NULL,
	PRIMARY KEY (app_token_id, origin)
);
