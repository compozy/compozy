-- +goose Up
-- create "computers" table
CREATE TABLE `computers` (`id` text NOT NULL, `account_id` text NOT NULL, `name` text NOT NULL, `enabled` integer NOT NULL DEFAULT 1, `platform` text NULL, `version` text NULL, `tls_root_sha256` text NOT NULL, `weight` integer NOT NULL DEFAULT 50, `added_at` text NOT NULL, `last_connected_at` text NULL, PRIMARY KEY (`id`), CHECK (weight IN (0, 25, 50, 100)));
-- create index "computers_account_id" to table: "computers"
CREATE INDEX `computers_account_id` ON `computers` (`account_id`, `id`);
-- create "computer_routes" table
CREATE TABLE `computer_routes` (`id` text NOT NULL, `computer_id` text NOT NULL, `kind` text NOT NULL, `address` text NOT NULL, `position` integer NOT NULL, `learned` integer NOT NULL DEFAULT 0, `removed` integer NOT NULL DEFAULT 0, `ssh_user` text NULL, `ssh_port` integer NULL, `ssh_alias` text NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`computer_id`) REFERENCES `computers` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (kind IN ('direct', 'ssh', 'cloud', 'relay')));
-- create index "computer_routes_computer_id_kind_address" to table: "computer_routes"
CREATE UNIQUE INDEX `computer_routes_computer_id_kind_address` ON `computer_routes` (`computer_id`, `kind`, `address`);
