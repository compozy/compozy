-- +goose Up

CREATE INDEX idx_migration_items_value ON migration_items(value);

INSERT INTO migration_items (id, value) VALUES (2, 'second');
