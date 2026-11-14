-- +goose Up
-- create "mcp_server_approvals" table
CREATE TABLE `mcp_server_approvals` (`id` text NULL, `profile_id` text NOT NULL, `workspace_id` text NOT NULL, `server_name` text NOT NULL, `source_id` text NOT NULL, `definition_digest` text NOT NULL, `approved_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE);
-- create index "mcp_server_approvals_profile_id_workspace_id_server_name_source_id" to table: "mcp_server_approvals"
CREATE UNIQUE INDEX `mcp_server_approvals_profile_id_workspace_id_server_name_source_id` ON `mcp_server_approvals` (`profile_id`, `workspace_id`, `server_name`, `source_id`);
