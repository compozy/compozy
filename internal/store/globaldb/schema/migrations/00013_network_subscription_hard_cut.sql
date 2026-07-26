-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create the hard-cut subscription table without digest or keyword filters
CREATE TABLE `new_network_subscriptions` (
  `workspace_id` text NOT NULL,
  `channel` text NOT NULL,
  `thread_id` text NOT NULL DEFAULT '',
  `session_id` text NOT NULL,
  `mode` text NOT NULL,
  `created_at` text NOT NULL,
  `updated_at` text NOT NULL,
  PRIMARY KEY (`workspace_id`, `channel`, `thread_id`, `session_id`),
  CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `channel`) REFERENCES `network_channels` (`workspace_id`, `channel`) ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE,
  CHECK (mode IN ('mute', 'full'))
);
-- preserve only canonical full/mute preferences; obsolete digest rows are deleted
INSERT INTO `new_network_subscriptions` (
  `workspace_id`, `channel`, `thread_id`, `session_id`, `mode`, `created_at`, `updated_at`
)
SELECT
  `workspace_id`, `channel`, `thread_id`, `session_id`, `mode`, `created_at`, `updated_at`
FROM `network_subscriptions`
WHERE `mode` IN ('mute', 'full');
-- replace the obsolete subscription table
DROP TABLE `network_subscriptions`;
ALTER TABLE `new_network_subscriptions` RENAME TO `network_subscriptions`;
CREATE INDEX `idx_network_subscriptions_session`
  ON `network_subscriptions` (`workspace_id`, `session_id`, `channel`, `thread_id`);
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
