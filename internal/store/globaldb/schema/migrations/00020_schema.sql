-- +goose Up
-- create "automation_suggestions" table
CREATE TABLE `automation_suggestions` (`id` text NULL, `workspace_id` text NOT NULL, `source` text NOT NULL, `dedup_key` text NOT NULL, `status` text NOT NULL, `payload_json` text NOT NULL, `created_at` text NOT NULL, `resolved_at` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (source IN ('catalog', 'usage', 'integration')), CHECK (status IN ('pending', 'accepted', 'dismissed')), CHECK (json_valid(payload_json) AND json_type(payload_json) = 'object'), CHECK (
			(status = 'pending' AND resolved_at IS NULL) OR
			(status IN ('accepted', 'dismissed') AND resolved_at IS NOT NULL)
		));
-- create index "automation_suggestions_workspace_id_dedup_key" to table: "automation_suggestions"
CREATE UNIQUE INDEX `automation_suggestions_workspace_id_dedup_key` ON `automation_suggestions` (`workspace_id`, `dedup_key`);
-- create index "idx_automation_suggestions_workspace_status" to table: "automation_suggestions"
CREATE INDEX `idx_automation_suggestions_workspace_status` ON `automation_suggestions` (`workspace_id`, `status`, `created_at`, `id`);
