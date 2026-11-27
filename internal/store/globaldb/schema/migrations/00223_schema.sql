-- +goose Up
-- create "remote_device_connections" table
CREATE TABLE `remote_device_connections` (`device_id` text NOT NULL, `last_seen_at` integer NOT NULL, PRIMARY KEY (`device_id`));

INSERT INTO remote_device_connections (device_id, last_seen_at)
SELECT device_id, MAX(created_at)
FROM remote_sessions
GROUP BY device_id;
