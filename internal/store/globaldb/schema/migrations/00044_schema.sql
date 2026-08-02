-- +goose Up
-- create index "idx_loop_generation_outputs_retry_due" to table: "loop_generation_outputs"
CREATE INDEX `idx_loop_generation_outputs_retry_due` ON `loop_generation_outputs` (`next_attempt_at`, `loop_run_id`, `generation`, `node_id`, `item_index`) WHERE status = 'retrying' AND next_attempt_at IS NOT NULL;
