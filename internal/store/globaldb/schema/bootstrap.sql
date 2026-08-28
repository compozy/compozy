INSERT INTO scheduler_pause (id, paused, paused_by, reason)
VALUES (1, 0, '', '');

INSERT INTO network_availability (id, enabled, epoch, updated_at, updated_by)
VALUES (1, 1, 1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), 'migration:00004');

INSERT INTO profiles (id, name, color, icon, emoji, state, created_at, archived_at)
VALUES ('00000000000000000000000000', 'default', '#8E8EB5', 'circle', NULL, 'active', '1970-01-01T00:00:00Z', NULL);

INSERT INTO model_catalog_execution_contexts (
	context_id, scope, profile_id, workspace_id, command_fingerprint
)
VALUES (
	'f4e0af598701912cd0516d11908757cf6c4f7a694c3113922f1f77d0fbd8c80e',
	'global', '', '', ''
);
