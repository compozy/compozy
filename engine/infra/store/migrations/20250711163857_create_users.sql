-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id         text NOT NULL PRIMARY KEY,
    email      text NOT NULL UNIQUE,
    role       text NOT NULL CHECK (role IN ('admin', 'user')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_ci ON users (lower(email));
CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
