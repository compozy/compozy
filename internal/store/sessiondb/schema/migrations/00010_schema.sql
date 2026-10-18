-- +goose Up
-- create index "idx_events_active_sequence" to table: "events"
CREATE INDEX `idx_events_active_sequence` ON `events` (`sequence`) WHERE archived = 0;
