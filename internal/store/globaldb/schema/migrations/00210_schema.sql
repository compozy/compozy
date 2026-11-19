-- +goose Up
-- create "extension_builtin_tombstones" table
CREATE TABLE `extension_builtin_tombstones` (`name` text NOT NULL, `removed_at` text NOT NULL, `removed_by` text NOT NULL, PRIMARY KEY (`name`), CHECK (trim(name) <> ''));
