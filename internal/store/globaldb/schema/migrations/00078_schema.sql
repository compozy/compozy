-- +goose Up
-- create "tool_approval_pending" table
CREATE TABLE `tool_approval_pending` (`approval_id` text NOT NULL, `workspace_id` text NOT NULL, `invocation_id` text NOT NULL, `target_kind` text NOT NULL, `tool_id` text NULL, `target_json` text NOT NULL DEFAULT '{}', `command_id` text NULL, `args_json` text NOT NULL, `approval_status` text NOT NULL, `execution_status` text NULL, `result_json` text NULL, `error_json` text NULL, `requested_at` integer NOT NULL, `expires_at` integer NOT NULL, `resolved_at` integer NULL, `executed_at` integer NULL, `resume_fence` integer NOT NULL DEFAULT 0, PRIMARY KEY (`approval_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (approval_id LIKE 'apr_%'), CHECK (trim(invocation_id) <> ''), CHECK (target_kind IN ('tool', 'client_op', 'navigate', 'view')), CHECK (json_valid(target_json)), CHECK (json_valid(args_json)), CHECK (
		approval_status IN ('pending', 'approved', 'denied', 'timeout', 'canceled')
	), CHECK (
		execution_status IS NULL OR execution_status IN ('dispatching', 'completed', 'failed', 'uncertain')
	), CHECK (result_json IS NULL OR json_valid(result_json)), CHECK (error_json IS NULL OR json_valid(error_json)), CHECK (expires_at > requested_at), CHECK (resume_fence IN (0, 1)), CHECK ((target_kind = 'tool' AND trim(coalesce(tool_id, '')) <> '') OR target_kind <> 'tool'), CHECK ((approval_status = 'pending' AND resolved_at IS NULL) OR (approval_status <> 'pending' AND resolved_at IS NOT NULL)), CHECK ((execution_status IS NULL AND executed_at IS NULL) OR execution_status IS NOT NULL));
-- create index "tool_approval_pending_invocation_id" to table: "tool_approval_pending"
CREATE UNIQUE INDEX `tool_approval_pending_invocation_id` ON `tool_approval_pending` (`invocation_id`);
-- create index "idx_tool_approval_pending_workspace_status" to table: "tool_approval_pending"
CREATE INDEX `idx_tool_approval_pending_workspace_status` ON `tool_approval_pending` (`workspace_id`, `approval_status`, `expires_at`, `approval_id`);
-- create index "idx_tool_approval_pending_recovery" to table: "tool_approval_pending"
CREATE INDEX `idx_tool_approval_pending_recovery` ON `tool_approval_pending` (`approval_status`, `execution_status`, `expires_at`, `resume_fence`);
