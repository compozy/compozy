-- +goose Up

CREATE TABLE migration_items (
    id INTEGER PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO migration_items (id, value) VALUES (1, 'first');
