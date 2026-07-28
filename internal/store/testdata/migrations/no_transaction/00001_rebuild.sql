-- +goose NO TRANSACTION
-- +goose Up

PRAGMA foreign_keys = OFF;

CREATE TABLE rebuild_items (
    id INTEGER PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO rebuild_items (id, value) VALUES (1, 'preserved');

CREATE TABLE rebuild_items_new (
    id INTEGER PRIMARY KEY,
    value TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1
);

INSERT INTO rebuild_items_new (id, value) SELECT id, value FROM rebuild_items;
DROP TABLE rebuild_items;
ALTER TABLE rebuild_items_new RENAME TO rebuild_items;

PRAGMA foreign_keys = ON;
