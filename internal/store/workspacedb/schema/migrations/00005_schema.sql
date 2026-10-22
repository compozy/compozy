-- +goose Up
-- create "legacy_import" table
CREATE TABLE `legacy_import` (`source_workspace_id` text NOT NULL, `source_row_id` text NOT NULL, `content_digest` text NOT NULL, PRIMARY KEY (`source_workspace_id`, `source_row_id`));
