-- +goose Up
-- create "cmd_palette_usage" table
CREATE TABLE `cmd_palette_usage` (`workspace_id` text NOT NULL, `command_id` text NOT NULL, `use_count` integer NOT NULL DEFAULT 0, `frecency_weight` real NOT NULL DEFAULT 0, `last_used_at` integer NOT NULL, `updated_at` integer NOT NULL, PRIMARY KEY (`workspace_id`, `command_id`), CHECK (trim(command_id) <> ''), CHECK (use_count >= 0), CHECK (frecency_weight >= 0), CHECK (last_used_at >= 0), CHECK (updated_at >= 0));
-- create index "idx_cmd_palette_usage_recents" to table: "cmd_palette_usage"
CREATE INDEX `idx_cmd_palette_usage_recents` ON `cmd_palette_usage` (`workspace_id`, `last_used_at` DESC, `command_id`);
-- create "cmd_palette_query_hits" table
CREATE TABLE `cmd_palette_query_hits` (`workspace_id` text NOT NULL, `query` text NOT NULL, `command_id` text NOT NULL, `weight` real NOT NULL DEFAULT 0, `last_used_at` integer NOT NULL, PRIMARY KEY (`workspace_id`, `query`, `command_id`), CHECK (trim(query) <> ''), CHECK (trim(command_id) <> ''), CHECK (weight >= 0), CHECK (last_used_at >= 0));
-- create index "idx_cmd_palette_query_hits_lookup" to table: "cmd_palette_query_hits"
CREATE INDEX `idx_cmd_palette_query_hits_lookup` ON `cmd_palette_query_hits` (`workspace_id`, `query`, `last_used_at` DESC, `command_id`);
-- create "cmd_palette_pins" table
CREATE TABLE `cmd_palette_pins` (`workspace_id` text NOT NULL, `command_id` text NOT NULL, `pinned_at` integer NOT NULL, PRIMARY KEY (`workspace_id`, `command_id`), CHECK (trim(command_id) <> ''), CHECK (pinned_at >= 0));
-- create index "idx_cmd_palette_pins_order" to table: "cmd_palette_pins"
CREATE INDEX `idx_cmd_palette_pins_order` ON `cmd_palette_pins` (`workspace_id`, `pinned_at`, `command_id`);
-- create trigger "cmd_palette_workspace_delete"
-- +goose StatementBegin
CREATE TRIGGER cmd_palette_workspace_delete
AFTER DELETE ON workspaces
BEGIN
	DELETE FROM cmd_palette_usage WHERE workspace_id = OLD.id;
	DELETE FROM cmd_palette_query_hits WHERE workspace_id = OLD.id;
	DELETE FROM cmd_palette_pins WHERE workspace_id = OLD.id;
END;
-- +goose StatementEnd
