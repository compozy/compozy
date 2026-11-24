-- +goose Up
-- add column "ssh_tunnel_address" to table: "computer_routes"
ALTER TABLE `computer_routes` ADD COLUMN `ssh_tunnel_address` text NULL;
-- add column "ssh_service_warning" to table: "computer_routes"
ALTER TABLE `computer_routes` ADD COLUMN `ssh_service_warning` text NULL;
-- create "computer_ssh_setups" table
CREATE TABLE `computer_ssh_setups` (`id` text NOT NULL, `account_id` text NOT NULL, `host_key` text NOT NULL, `target_alias` text NOT NULL, `name` text NOT NULL, `kind` text NOT NULL, `step` text NOT NULL, `release_version` text NOT NULL, `platform` text NOT NULL DEFAULT '', `binary_path` text NOT NULL DEFAULT '', `service_warning` text NOT NULL DEFAULT '', `reservation_id` text NOT NULL DEFAULT '', `link_token_ref` text NOT NULL DEFAULT '', `link_token_expires_at` text NOT NULL DEFAULT '', `computer_id` text NOT NULL DEFAULT '', `error_code` text NOT NULL DEFAULT '', `error_detail` text NOT NULL DEFAULT '', `updated_at` text NOT NULL, PRIMARY KEY (`id`), CHECK (kind IN ('add', 'update')), CHECK (step IN ('admit', 'verify', 'connect', 'install', 'service', 'link', 'forward', 'done', 'cancelled')));
-- create index "computer_ssh_setups_pending_host" to table: "computer_ssh_setups"
CREATE UNIQUE INDEX `computer_ssh_setups_pending_host` ON `computer_ssh_setups` (`account_id`, `host_key`) WHERE step NOT IN ('done', 'cancelled');
