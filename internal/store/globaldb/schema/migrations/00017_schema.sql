-- +goose Up
-- create "tool_approval_grants" table
CREATE TABLE `tool_approval_grants` (`id` text NOT NULL, `workspace_id` text NOT NULL, `agent_name` text NOT NULL DEFAULT '', `tool_id` text NOT NULL, `input_digest` text NOT NULL DEFAULT '', `decision` text NOT NULL, `created_at` text NOT NULL, `last_used_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (trim(id) <> ''), CHECK (trim(tool_id) <> ''), CHECK (input_digest = '' OR input_digest LIKE 'sha256:%'), CHECK (decision IN ('allow', 'reject')));
-- create index "tool_approval_grants_workspace_id_agent_name_tool_id_input_digest" to table: "tool_approval_grants"
CREATE UNIQUE INDEX `tool_approval_grants_workspace_id_agent_name_tool_id_input_digest` ON `tool_approval_grants` (`workspace_id`, `agent_name`, `tool_id`, `input_digest`);
-- create index "idx_tool_approval_grants_lookup" to table: "tool_approval_grants"
CREATE INDEX `idx_tool_approval_grants_lookup` ON `tool_approval_grants` (`workspace_id`, `tool_id`, `agent_name`, `input_digest`);
-- create index "idx_tool_approval_grants_list" to table: "tool_approval_grants"
CREATE INDEX `idx_tool_approval_grants_list` ON `tool_approval_grants` (`workspace_id`, `created_at` DESC, `id`);
