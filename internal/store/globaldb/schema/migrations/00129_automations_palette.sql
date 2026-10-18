-- +goose Up

INSERT INTO cmd_palette_usage (workspace_id, profile_lens_id, command_id, use_count, frecency_weight, last_used_at, updated_at)
SELECT workspace_id, profile_lens_id, CASE WHEN command_id IN ('app.open.jobs', 'app.open.triggers') THEN 'app.open.automations' ELSE 'palette.view.automations' END, use_count, frecency_weight, last_used_at, updated_at
FROM cmd_palette_usage
WHERE command_id IN ('app.open.jobs', 'app.open.triggers', 'palette.view.jobs', 'palette.view.triggers')
ON CONFLICT (workspace_id, profile_lens_id, command_id) DO UPDATE SET
 use_count = cmd_palette_usage.use_count + excluded.use_count,
 frecency_weight = max(cmd_palette_usage.frecency_weight, excluded.frecency_weight),
 last_used_at = max(cmd_palette_usage.last_used_at, excluded.last_used_at),
 updated_at = max(cmd_palette_usage.updated_at, excluded.updated_at);
DELETE FROM cmd_palette_usage WHERE command_id IN ('app.open.jobs', 'app.open.triggers', 'palette.view.jobs', 'palette.view.triggers');

INSERT INTO cmd_palette_query_hits (workspace_id, profile_lens_id, query, command_id, weight, last_used_at)
SELECT workspace_id, profile_lens_id, query, CASE WHEN command_id IN ('app.open.jobs', 'app.open.triggers') THEN 'app.open.automations' ELSE 'palette.view.automations' END, weight, last_used_at
FROM cmd_palette_query_hits
WHERE command_id IN ('app.open.jobs', 'app.open.triggers', 'palette.view.jobs', 'palette.view.triggers')
ON CONFLICT (workspace_id, profile_lens_id, query, command_id) DO UPDATE SET
 weight = cmd_palette_query_hits.weight + excluded.weight,
 last_used_at = max(cmd_palette_query_hits.last_used_at, excluded.last_used_at);
DELETE FROM cmd_palette_query_hits WHERE command_id IN ('app.open.jobs', 'app.open.triggers', 'palette.view.jobs', 'palette.view.triggers');

INSERT INTO cmd_palette_pins (workspace_id, profile_lens_id, command_id, pinned_at)
SELECT workspace_id, profile_lens_id, CASE WHEN command_id IN ('app.open.jobs', 'app.open.triggers') THEN 'app.open.automations' ELSE 'palette.view.automations' END, pinned_at
FROM cmd_palette_pins
WHERE command_id IN ('app.open.jobs', 'app.open.triggers', 'palette.view.jobs', 'palette.view.triggers')
ON CONFLICT (workspace_id, profile_lens_id, command_id) DO UPDATE SET
 pinned_at = min(cmd_palette_pins.pinned_at, excluded.pinned_at);
DELETE FROM cmd_palette_pins WHERE command_id IN ('app.open.jobs', 'app.open.triggers', 'palette.view.jobs', 'palette.view.triggers');

-- +goose Down
-- Merged personalization cannot be split back into its former command identities.
SELECT 1;
