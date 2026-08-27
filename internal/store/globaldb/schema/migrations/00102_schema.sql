-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_call_deliveries" table
CREATE TABLE `new_call_deliveries` (`delivery_id` text NULL, `kind` text NOT NULL, `subject_id` text NOT NULL, `recipient_session_id` text NOT NULL, `owner_key` text NOT NULL, `wake_event_id` text NOT NULL, `state` text NOT NULL, `reason` text NOT NULL DEFAULT '', `attempts` integer NOT NULL DEFAULT 0, `created_at` text NOT NULL, `updated_at` text NOT NULL, `delivered_at` text NULL, PRIMARY KEY (`delivery_id`), CONSTRAINT `0` FOREIGN KEY (`recipient_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (delivery_id LIKE 'delivery_%'), CHECK (kind IN ('completion', 'follow-up', 'message', 'repair')), CHECK (trim(subject_id) <> ''), CHECK (trim(owner_key) <> ''), CHECK (trim(wake_event_id) <> ''), CHECK (state IN ('pending', 'attention', 'injected', 'woken', 'failed')), CHECK (attempts >= 0));
-- copy rows from old table "call_deliveries" to new temporary table "new_call_deliveries"
INSERT INTO `new_call_deliveries` (`delivery_id`, `kind`, `subject_id`, `recipient_session_id`, `owner_key`, `wake_event_id`, `state`, `reason`, `attempts`, `created_at`, `updated_at`, `delivered_at`) SELECT `delivery_id`, `kind`, `subject_id`, `recipient_session_id`, `owner_key`, `wake_event_id`, `state`, `reason`, `attempts`, `created_at`, `updated_at`, `delivered_at` FROM `call_deliveries`;
-- drop "call_deliveries" table after copying rows
DROP TABLE `call_deliveries`;
-- rename temporary table "new_call_deliveries" to "call_deliveries"
ALTER TABLE `new_call_deliveries` RENAME TO `call_deliveries`;
-- create index "call_deliveries_wake_event_id" to table: "call_deliveries"
CREATE UNIQUE INDEX `call_deliveries_wake_event_id` ON `call_deliveries` (`wake_event_id`);
-- create index "call_deliveries_kind_subject_id_recipient_session_id" to table: "call_deliveries"
CREATE UNIQUE INDEX `call_deliveries_kind_subject_id_recipient_session_id` ON `call_deliveries` (`kind`, `subject_id`, `recipient_session_id`);
-- create index "idx_call_deliveries_pending" to table: "call_deliveries"
CREATE INDEX `idx_call_deliveries_pending` ON `call_deliveries` (`state`, `created_at`, `delivery_id`);
-- create index "idx_call_deliveries_recipient" to table: "call_deliveries"
CREATE INDEX `idx_call_deliveries_recipient` ON `call_deliveries` (`recipient_session_id`, `state`, `created_at`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
