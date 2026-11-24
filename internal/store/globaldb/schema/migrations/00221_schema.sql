-- +goose Up
-- create "installation_account" table
CREATE TABLE `installation_account` (`id` integer NULL, `installation_id` text NOT NULL, `pre_account_install` integer NOT NULL, `state` text NOT NULL, `source` text NULL, `account_id` text NULL, `email` text NULL, `name` text NULL, `provider` text NULL, `device_id` text NULL, `device_name` text NULL, `device_admin` integer NULL, `marketing_opt_in` integer NOT NULL DEFAULT 0, `plan_json` text NULL, `signed_out_reason` text NULL, `auth_generation` integer NOT NULL DEFAULT 0, `last_validated_at` text NULL, PRIMARY KEY (`id`), CHECK (id = 1), CHECK (pre_account_install IN (0, 1)), CHECK (state IN ('signed_out', 'signed_in', 'reauth_required')), CHECK (source IN ('session', 'link', 'token')), CHECK (device_admin IN (0, 1)), CHECK (marketing_opt_in IN (0, 1)), CHECK (auth_generation >= 0));
INSERT INTO installation_account (id, installation_id, pre_account_install, state)
SELECT 1, 'ins_' || lower(hex(randomblob(16))),
  EXISTS (SELECT 1 FROM sessions)
    OR EXISTS (SELECT 1 FROM workspaces)
    OR EXISTS (SELECT 1 FROM automation_jobs),
  'signed_out';
