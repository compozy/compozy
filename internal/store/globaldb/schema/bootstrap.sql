INSERT INTO scheduler_pause (id, paused, paused_by, reason)
VALUES (1, 0, '', '');

INSERT INTO network_availability (id, enabled, epoch, updated_at, updated_by)
VALUES (1, 1, 1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), 'migration:00004');
