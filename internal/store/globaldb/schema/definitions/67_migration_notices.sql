CREATE TABLE migration_notices (
	key TEXT PRIMARY KEY,
	payload TEXT NOT NULL CHECK (json_valid(payload)),
	created_at TEXT NOT NULL,
	dismissed_at TEXT
);
