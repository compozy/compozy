-- +goose Up
-- create "dead_entities" table
CREATE TABLE `dead_entities` (`workspace_id` text NOT NULL, `kind` text NOT NULL, `entity_id` text NOT NULL, `reason` text NOT NULL, `marked_at` text NOT NULL, PRIMARY KEY (`workspace_id`, `kind`, `entity_id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`) REFERENCES `workspaces` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (kind IN ('extension', 'bridge', 'mcp_sidecar')), CHECK (trim(entity_id) <> ''), CHECK (trim(reason) <> ''));

