-- +goose Up
-- create index "idx_sessions_side" to table: "sessions"
CREATE INDEX `idx_sessions_side` ON `sessions` (`lineage_kind`) WHERE lineage_kind = 'side';
