-- +goose Up
-- +goose StatementBegin
-- Note: When updating last_used, use "last_used = GREATEST(last_used, now())" 
-- in the repository layer to prevent race condition overwrites
CREATE TABLE IF NOT EXISTS api_keys (
    id         text NOT NULL PRIMARY KEY,
    user_id    text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hash       bytea NOT NULL,
    prefix     text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used  timestamptz
);

CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys (hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys (user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_created_at ON api_keys (created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys (prefix);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS api_keys;
-- +goose StatementEnd
