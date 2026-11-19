-- +goose Up
-- create "operator_notice_outbox" table
CREATE TABLE `operator_notice_outbox` (`id` text NOT NULL, `profile_id` text NOT NULL, `workspace_id` text NULL, `session_id` text NOT NULL, `kind` text NOT NULL, `action` text NOT NULL, `resource` text NOT NULL, `title` text NOT NULL, `body` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE);
-- create index "operator_notice_outbox_profile_idx" to table: "operator_notice_outbox"
CREATE INDEX `operator_notice_outbox_profile_idx` ON `operator_notice_outbox` (`profile_id`, `created_at`);
-- create index "operator_notice_outbox_workspace_idx" to table: "operator_notice_outbox"
CREATE INDEX `operator_notice_outbox_workspace_idx` ON `operator_notice_outbox` (`workspace_id`);
