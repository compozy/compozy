-- +goose Up
-- create "session_input_clear_traces" table
CREATE TABLE `session_input_clear_traces` (`entry_id` text NULL, `session_id` text NOT NULL, `turn_id` text NOT NULL, `actor_kind` text NOT NULL, `actor_id` text NOT NULL, `queue_generation` integer NOT NULL, `created_at` text NOT NULL, `projected_at` text NULL, PRIMARY KEY (`entry_id`), CONSTRAINT `0` FOREIGN KEY (`session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`entry_id`) REFERENCES `session_input_queue` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(actor_kind)) > 0), CHECK (length(trim(actor_id)) > 0));
-- create index "idx_session_input_clear_traces_pending" to table: "session_input_clear_traces"
CREATE INDEX `idx_session_input_clear_traces_pending` ON `session_input_clear_traces` (`session_id`, `created_at`, `entry_id`) WHERE projected_at IS NULL;
