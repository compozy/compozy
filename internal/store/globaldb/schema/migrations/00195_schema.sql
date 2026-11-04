-- +goose Up
-- create index "idx_token_usage_daily_activity" to table: "token_usage_daily"
CREATE INDEX `idx_token_usage_daily_activity` ON `token_usage_daily` (`profile_id`, `agent_name`, `workspace_id`, `day`);
-- create index "idx_agent_instances_activity" to table: "agent_instances"
CREATE INDEX `idx_agent_instances_activity` ON `agent_instances` (`profile_id`, `agent_name`, `workspace_id`);
-- create index "idx_agent_wake_events_activity" to table: "agent_heartbeat_wake_events"
CREATE INDEX `idx_agent_wake_events_activity` ON `agent_heartbeat_wake_events` (`instance_id`, `result`, `created_at`);
