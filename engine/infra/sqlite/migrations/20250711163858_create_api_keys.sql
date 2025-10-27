-- +goose Up
-- +goose StatementBegin
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS api_keys (
    id         TEXT NOT NULL PRIMARY KEY,
    user_id    TEXT NOT NULL,
    hash       BLOB NOT NULL,
    prefix     TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    last_used  TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys (hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys (user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_created_at ON api_keys (created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS api_keys;
-- +goose StatementEnd
