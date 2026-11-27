-- +goose Up
CREATE TABLE gate_policy (
    id smallint PRIMARY KEY CHECK (id = 1),
    mode text NOT NULL CHECK (mode IN ('enforce', 'grace')),
    until timestamptz,
    message text NOT NULL DEFAULT '',
    updated_by text NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK ((mode = 'enforce' AND until IS NULL) OR (mode = 'grace' AND until IS NOT NULL))
);

INSERT INTO gate_policy (id, mode, updated_by, updated_at) VALUES (1, 'enforce', 'system', now());
