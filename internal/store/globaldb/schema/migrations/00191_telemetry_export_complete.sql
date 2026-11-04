-- +goose Up
-- Every source row must have a verified, FULL-synchronous commit in the telemetry
-- database before this boundary can remove the emptied export table.
CREATE TEMP TABLE telemetry_export_guard (remaining INTEGER CHECK (remaining = 0));
INSERT INTO telemetry_export_guard SELECT COUNT(*) FROM event_summary_export;
DROP TABLE event_summary_export;
DROP TABLE telemetry_export_guard;
