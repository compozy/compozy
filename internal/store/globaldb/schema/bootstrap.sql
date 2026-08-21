INSERT INTO scheduler_pause (id, paused, paused_by, reason)
VALUES (1, 0, '', '');

INSERT INTO network_availability (id, enabled, epoch, updated_at, updated_by)
VALUES (1, 1, 1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), 'migration:00004');

INSERT INTO profiles (id, name, color, icon, emoji, state, created_at, archived_at)
VALUES ('00000000000000000000000000', 'default', '#8E8EB5', 'circle', NULL, 'active', '1970-01-01T00:00:00Z', NULL);
