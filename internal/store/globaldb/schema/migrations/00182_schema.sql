-- +goose Up
-- create index "idx_session_prompt_admissions_human_turn" to table: "session_prompt_admissions"
CREATE INDEX `idx_session_prompt_admissions_human_turn` ON `session_prompt_admissions` (`session_id`, `turn_id`) WHERE human_ingress = 1 AND dispatch_committed_at IS NOT NULL;
