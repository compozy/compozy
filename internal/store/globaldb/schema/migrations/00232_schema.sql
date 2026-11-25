-- +goose Up
-- add column "lease_sequence" to table: "remote_identity"
ALTER TABLE `remote_identity` ADD COLUMN `lease_sequence` integer NOT NULL DEFAULT 0;
-- add column "last_mint_at" to table: "remote_identity"
ALTER TABLE `remote_identity` ADD COLUMN `last_mint_at` integer NOT NULL DEFAULT 0;
-- add column "last_mint_device_id" to table: "remote_identity"
ALTER TABLE `remote_identity` ADD COLUMN `last_mint_device_id` text NOT NULL DEFAULT '';
-- add column "last_mint_jkt" to table: "remote_identity"
ALTER TABLE `remote_identity` ADD COLUMN `last_mint_jkt` text NOT NULL DEFAULT '';

UPDATE remote_identity
SET (last_mint_at, last_mint_device_id, last_mint_jkt) = (
    SELECT created_at / 1000000000, device_id, device_jkt
    FROM remote_sessions
    ORDER BY created_at DESC, id DESC
    LIMIT 1
)
WHERE EXISTS (SELECT 1 FROM remote_sessions);
