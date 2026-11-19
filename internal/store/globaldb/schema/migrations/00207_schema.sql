-- +goose Up
-- create "token_usage_receipts" table
CREATE TABLE `token_usage_receipts` (`profile_id` text NOT NULL, `session_id` text NOT NULL, `turn_id` text NOT NULL, `instance_id` text NOT NULL DEFAULT '', `day` text NOT NULL, `total_tokens` integer NOT NULL DEFAULT 0, `usd_amount` text NULL, `cost_status` text NOT NULL, `created_at` text NOT NULL, PRIMARY KEY (`session_id`, `turn_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(trim(turn_id)) > 0), CHECK (total_tokens >= 0));
-- create index "idx_token_usage_receipts_instance_day" to table: "token_usage_receipts"
CREATE INDEX `idx_token_usage_receipts_instance_day` ON `token_usage_receipts` (`profile_id`, `instance_id`, `day`);
