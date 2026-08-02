-- +goose Up
-- disable the enforcement of foreign-key constraints
PRAGMA foreign_keys = off;
-- rebuild admission claims so existing rows receive the prior default horizon
CREATE TABLE `new_loop_admission_claims` (
  `workspace_id` text NOT NULL,
  `loop_name` text NOT NULL,
  `source_key` text NOT NULL,
  `event_key` text NOT NULL,
  `loop_run_id` text NOT NULL,
  `claimed_at` timestamp NOT NULL,
  `expires_at` timestamp NOT NULL,
  `suppressed_count` integer NOT NULL DEFAULT 0,
  `last_suppressed_at` timestamp NULL,
  PRIMARY KEY (`workspace_id`, `loop_name`, `source_key`, `event_key`)
);
INSERT INTO `new_loop_admission_claims` (
  `workspace_id`, `loop_name`, `source_key`, `event_key`, `loop_run_id`, `claimed_at`,
  `expires_at`, `suppressed_count`, `last_suppressed_at`
)
SELECT `workspace_id`, `loop_name`, `source_key`, `event_key`, `loop_run_id`, `claimed_at`,
  datetime(`claimed_at`, '+168 hours'), `suppressed_count`, `last_suppressed_at`
FROM `loop_admission_claims`;
DROP TABLE `loop_admission_claims`;
ALTER TABLE `new_loop_admission_claims` RENAME TO `loop_admission_claims`;
-- enable the enforcement of foreign-key constraints
PRAGMA foreign_keys = on;
