-- +goose Up
-- create "session_folders" table
CREATE TABLE `session_folders` (`id` text NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL, `name` text NOT NULL, `position` integer NOT NULL, `created_at` text NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(id)) > 0), CHECK (length(trim(name)) > 0 AND length(name) <= 120), CHECK (position >= 0));
-- create index "session_folders_profile_id_workspace_id_id" to table: "session_folders"
CREATE UNIQUE INDEX `session_folders_profile_id_workspace_id_id` ON `session_folders` (`profile_id`, `workspace_id`, `id`);
-- create index "idx_session_folders_scope" to table: "session_folders"
CREATE INDEX `idx_session_folders_scope` ON `session_folders` (`profile_id`, `workspace_id`, `position`);
-- create "session_placements" table
CREATE TABLE `session_placements` (`session_id` text NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL, `folder_id` text NULL, `position` integer NOT NULL, `updated_at` text NOT NULL, PRIMARY KEY (`session_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`, `workspace_id`, `folder_id`) REFERENCES `session_folders` (`profile_id`, `workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (position >= 0));
-- create index "idx_session_placements_container" to table: "session_placements"
CREATE INDEX `idx_session_placements_container` ON `session_placements` (`profile_id`, `workspace_id`, `folder_id`, `position`);
-- create index "idx_session_placements_folder" to table: "session_placements"
CREATE INDEX `idx_session_placements_folder` ON `session_placements` (`folder_id`);
