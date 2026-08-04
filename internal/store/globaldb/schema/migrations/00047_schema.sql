-- +goose Up
-- Rebuild the table so pre-existing tombstones receive the pinned default
-- horizon without leaving a permanent column default in the fresh schema.
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
  `workspace_id`, `loop_name`, `source_key`, `event_key`, `loop_run_id`,
  `claimed_at`, `expires_at`, `suppressed_count`, `last_suppressed_at`
)
SELECT
  `workspace_id`, `loop_name`, `source_key`, `event_key`, `loop_run_id`,
  `claimed_at`, COALESCE(datetime(`claimed_at`, '+7 days'), `claimed_at`),
  `suppressed_count`, `last_suppressed_at`
FROM `loop_admission_claims`;
DROP TABLE `loop_admission_claims`;
ALTER TABLE `new_loop_admission_claims` RENAME TO `loop_admission_claims`;
-- create index "idx_loop_admission_claims_expiry" to table: "loop_admission_claims"
CREATE INDEX `idx_loop_admission_claims_expiry` ON `loop_admission_claims` (`expires_at`);
