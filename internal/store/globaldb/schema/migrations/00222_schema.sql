-- +goose Up
-- create "remote_identity" table
CREATE TABLE `remote_identity` (`id` integer NULL, `computer_id` text NOT NULL, `account_id` text NOT NULL, `computer_key_ref` text NOT NULL, `tls_root_ref` text NOT NULL, `tls_root_sha256` text NOT NULL, `tls_leaf_not_after` text NOT NULL, `linked_at` text NOT NULL, PRIMARY KEY (`id`), CHECK (id = 1));
-- create "remote_sessions" table
CREATE TABLE `remote_sessions` (`id` text NULL, `credential_sha256` blob NOT NULL, `device_id` text NOT NULL, `device_jkt` text NOT NULL, `scopes` text NOT NULL, `route` text NOT NULL, `browser` integer NOT NULL, `created_at` integer NOT NULL, `expires_at` integer NOT NULL, `revoked_at` integer NULL, PRIMARY KEY (`id`));
-- create index "remote_sessions_credential_sha256" to table: "remote_sessions"
CREATE UNIQUE INDEX `remote_sessions_credential_sha256` ON `remote_sessions` (`credential_sha256`);
-- create index "idx_remote_sessions_device" to table: "remote_sessions"
CREATE INDEX `idx_remote_sessions_device` ON `remote_sessions` (`device_id`);
-- create index "idx_remote_sessions_expiry" to table: "remote_sessions"
CREATE INDEX `idx_remote_sessions_expiry` ON `remote_sessions` (`expires_at`);
-- create "remote_grant_replay" table
CREATE TABLE `remote_grant_replay` (`jti` text NULL, `expires_at` integer NOT NULL, PRIMARY KEY (`jti`));
-- create "remote_device_approvals" table
CREATE TABLE `remote_device_approvals` (`id` text NULL, `device_id` text NOT NULL, `jkt` text NOT NULL, `name` text NOT NULL, `state` text NOT NULL, `created_at` integer NOT NULL, `expires_at` integer NOT NULL, `decided_at` integer NULL, `decision_jti` text NOT NULL DEFAULT '', PRIMARY KEY (`id`), CHECK (state IN ('pending', 'approved', 'denied', 'expired')));
-- create index "remote_device_approvals_device_id_jkt" to table: "remote_device_approvals"
CREATE UNIQUE INDEX `remote_device_approvals_device_id_jkt` ON `remote_device_approvals` (`device_id`, `jkt`);
-- create "remote_approval_replay" table
CREATE TABLE `remote_approval_replay` (`jti` text NULL, `expires_at` integer NOT NULL, PRIMARY KEY (`jti`));
-- create "remote_security_state" table
CREATE TABLE `remote_security_state` (`id` integer NULL, `applied_epoch` integer NOT NULL, `devices_json` text NOT NULL, `applied_at` integer NOT NULL, PRIMARY KEY (`id`), CHECK (id = 1));
