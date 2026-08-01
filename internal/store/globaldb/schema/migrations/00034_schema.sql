-- +goose Up
-- add column "prompt_admission_id" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `prompt_admission_id` text NULL;
-- add column "message_id" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `message_id` text NOT NULL DEFAULT '';
-- add column "idempotency_key" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `idempotency_key` text NOT NULL DEFAULT '';
-- add column "turn_id" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `turn_id` text NOT NULL DEFAULT '';
-- add column "event_id" to table: "session_input_queue"
ALTER TABLE `session_input_queue` ADD COLUMN `event_id` text NOT NULL DEFAULT '';
-- create index "uq_session_input_queue_prompt_admission" to table: "session_input_queue"
CREATE UNIQUE INDEX `uq_session_input_queue_prompt_admission` ON `session_input_queue` (`prompt_admission_id`) WHERE prompt_admission_id IS NOT NULL;
-- create "session_prompt_admissions" table
CREATE TABLE `session_prompt_admissions` (`id` text NOT NULL, `workspace_id` text NOT NULL, `session_id` text NOT NULL, `message_id` text NOT NULL, `idempotency_key` text NOT NULL, `operation` text NOT NULL, `fingerprint_version` text NOT NULL, `request_fingerprint` text NOT NULL, `state` text NOT NULL, `mode` text NOT NULL DEFAULT '', `authored_text` text NOT NULL DEFAULT '', `runtime_provider` text NOT NULL DEFAULT '', `runtime_model` text NOT NULL DEFAULT '', `runtime_reasoning_effort` text NOT NULL DEFAULT '', `runtime_speed` text NOT NULL DEFAULT '', `turn_id` text NOT NULL, `event_id` text NOT NULL, `result_json` text NULL, `indeterminate_reason` text NOT NULL DEFAULT '', `created_at` text NOT NULL, `dispatch_committed_at` text NULL, `completed_at` text NULL, `updated_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`workspace_id`, `session_id`) REFERENCES `sessions` (`workspace_id`, `id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(id)) > 0), CHECK (length(trim(message_id)) > 0), CHECK (length(trim(idempotency_key)) > 0), CHECK (operation IN ('prompt', 'steer')), CHECK (length(trim(fingerprint_version)) > 0), CHECK (length(trim(request_fingerprint)) > 0), CHECK (state IN ('reserved', 'dispatch_committed', 'completed', 'indeterminate')), CHECK (length(trim(turn_id)) > 0), CHECK (length(trim(event_id)) > 0), CHECK (result_json IS NULL OR json_valid(result_json)));
-- create index "session_prompt_admissions_workspace_id_session_id_idempotency_key" to table: "session_prompt_admissions"
CREATE UNIQUE INDEX `session_prompt_admissions_workspace_id_session_id_idempotency_key` ON `session_prompt_admissions` (`workspace_id`, `session_id`, `idempotency_key`);
-- create index "session_prompt_admissions_workspace_id_session_id_message_id" to table: "session_prompt_admissions"
CREATE UNIQUE INDEX `session_prompt_admissions_workspace_id_session_id_message_id` ON `session_prompt_admissions` (`workspace_id`, `session_id`, `message_id`);
-- create index "idx_session_prompt_admissions_state" to table: "session_prompt_admissions"
CREATE INDEX `idx_session_prompt_admissions_state` ON `session_prompt_admissions` (`workspace_id`, `session_id`, `state`, `updated_at`);
