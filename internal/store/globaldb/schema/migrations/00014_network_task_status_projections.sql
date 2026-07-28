-- +goose Up
CREATE TABLE `network_task_status_projections` (
  `event_id` text NOT NULL,
  `recipient_session_id` text NOT NULL,
  `workspace_id` text NOT NULL,
  `channel` text NOT NULL,
  `thread_id` text NOT NULL,
  `task_id` text NOT NULL,
  `run_id` text NOT NULL DEFAULT '',
  `event_type` text NOT NULL,
  `projection_json` text NOT NULL,
  `projected_at` text NOT NULL,
  PRIMARY KEY (`event_id`, `recipient_session_id`),
  CONSTRAINT `0` FOREIGN KEY (`event_id`) REFERENCES `task_events` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT `1` FOREIGN KEY (`workspace_id`, `channel`, `thread_id`) REFERENCES `network_threads` (`workspace_id`, `channel`, `thread_id`) ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT `2` FOREIGN KEY (`task_id`) REFERENCES `tasks` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT `3` FOREIGN KEY (`workspace_id`, `recipient_session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE,
  CHECK (json_valid(projection_json))
);
CREATE INDEX `idx_network_task_status_projections_recipient` ON `network_task_status_projections` (`workspace_id`, `recipient_session_id`, `projected_at` DESC, `event_id`);
CREATE INDEX `idx_network_task_status_projections_task` ON `network_task_status_projections` (`workspace_id`, `task_id`, `projected_at` DESC, `event_id`);
