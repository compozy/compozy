-- +goose Up
-- create index "idx_token_usage_receipts_budget_cursor" to table: "token_usage_receipts"
CREATE INDEX `idx_token_usage_receipts_budget_cursor` ON `token_usage_receipts` (`profile_id`, `instance_id`, `day`, `session_id`, `turn_id`);
