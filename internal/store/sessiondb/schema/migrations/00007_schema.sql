-- +goose Up
-- create index "idx_transcript_entries_role_sequence" to table: "transcript_entries"
CREATE INDEX `idx_transcript_entries_role_sequence` ON `transcript_entries` (`kind`, `start_sequence`);
