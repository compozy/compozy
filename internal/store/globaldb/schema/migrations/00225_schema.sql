-- +goose Up
-- add column "sign_in_notices_dismissed_at" to table: "installation_account"
ALTER TABLE `installation_account` ADD COLUMN `sign_in_notices_dismissed_at` text NULL;
