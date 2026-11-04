-- +goose Up
-- The preceding migration created an empty staging shape. Refuse to replace it if another
-- writer populated it, then freeze the original ledger with a lossless rename.
CREATE TEMP TABLE telemetry_export_guard (remaining INTEGER CHECK (remaining = 0));
INSERT INTO telemetry_export_guard SELECT COUNT(*) FROM event_summary_export;
DROP TABLE event_summary_export;
DROP TABLE telemetry_export_guard;
ALTER TABLE event_summaries RENAME TO event_summary_export;
