-- +goose Up
-- add column "profile_id" to table: "agent_heartbeat_revisions"
ALTER TABLE `agent_heartbeat_revisions` ADD COLUMN `profile_id` text NOT NULL DEFAULT '00000000000000000000000000';
-- drop index "idx_agent_heartbeat_revisions_agent_created" from table: "agent_heartbeat_revisions"
DROP INDEX `idx_agent_heartbeat_revisions_agent_created`;
-- create index "idx_agent_heartbeat_revisions_agent_created" to table: "agent_heartbeat_revisions"
CREATE INDEX `idx_agent_heartbeat_revisions_agent_created` ON `agent_heartbeat_revisions` (`profile_id`, `workspace_id`, `agent_name`, `created_at` DESC, `id` DESC);
-- add column "profile_id" to table: "agent_heartbeat_snapshots"
ALTER TABLE `agent_heartbeat_snapshots` ADD COLUMN `profile_id` text NOT NULL DEFAULT '00000000000000000000000000';
-- drop index "agent_heartbeat_snapshots_workspace_id_agent_name_digest" from table: "agent_heartbeat_snapshots"
DROP INDEX `agent_heartbeat_snapshots_workspace_id_agent_name_digest`;
-- drop index "idx_agent_heartbeat_snapshots_agent_created" from table: "agent_heartbeat_snapshots"
DROP INDEX `idx_agent_heartbeat_snapshots_agent_created`;
-- create index "idx_agent_heartbeat_snapshots_agent_created" to table: "agent_heartbeat_snapshots"
CREATE INDEX `idx_agent_heartbeat_snapshots_agent_created` ON `agent_heartbeat_snapshots` (`profile_id`, `workspace_id`, `agent_name`, `created_at` DESC, `id` DESC);
-- create index "agent_heartbeat_snapshots_profile_id_workspace_id_agent_name_digest" to table: "agent_heartbeat_snapshots"
CREATE UNIQUE INDEX `agent_heartbeat_snapshots_profile_id_workspace_id_agent_name_digest` ON `agent_heartbeat_snapshots` (`profile_id`, `workspace_id`, `agent_name`, `digest`);
-- add column "profile_id" to table: "agent_soul_revisions"
ALTER TABLE `agent_soul_revisions` ADD COLUMN `profile_id` text NOT NULL DEFAULT '00000000000000000000000000';
-- drop index "idx_agent_soul_revisions_agent" from table: "agent_soul_revisions"
DROP INDEX `idx_agent_soul_revisions_agent`;
-- create index "idx_agent_soul_revisions_agent" to table: "agent_soul_revisions"
CREATE INDEX `idx_agent_soul_revisions_agent` ON `agent_soul_revisions` (`profile_id`, `workspace_id`, `agent_name`, `created_at` DESC);
-- add column "profile_id" to table: "agent_soul_snapshots"
ALTER TABLE `agent_soul_snapshots` ADD COLUMN `profile_id` text NOT NULL DEFAULT '00000000000000000000000000';
-- drop index "agent_soul_snapshots_workspace_id_agent_name_digest" from table: "agent_soul_snapshots"
DROP INDEX `agent_soul_snapshots_workspace_id_agent_name_digest`;
-- drop index "idx_agent_soul_snapshots_agent" from table: "agent_soul_snapshots"
DROP INDEX `idx_agent_soul_snapshots_agent`;
-- create index "idx_agent_soul_snapshots_agent" to table: "agent_soul_snapshots"
CREATE INDEX `idx_agent_soul_snapshots_agent` ON `agent_soul_snapshots` (`profile_id`, `workspace_id`, `agent_name`, `created_at` DESC);
-- create index "agent_soul_snapshots_profile_id_workspace_id_agent_name_digest" to table: "agent_soul_snapshots"
CREATE UNIQUE INDEX `agent_soul_snapshots_profile_id_workspace_id_agent_name_digest` ON `agent_soul_snapshots` (`profile_id`, `workspace_id`, `agent_name`, `digest`);

-- Preserve ownership recorded in profile-specific authored source paths.
UPDATE agent_soul_snapshots
SET profile_id = COALESCE((
  SELECT id FROM profiles
  WHERE instr('/' || replace(agent_soul_snapshots.source_path, char(92), '/'), '/profiles/' || profiles.name || '/agents/') > 0
), profile_id);
UPDATE agent_soul_revisions
SET profile_id = COALESCE((
  SELECT id FROM profiles
  WHERE instr('/' || replace(agent_soul_revisions.source_path, char(92), '/'), '/profiles/' || profiles.name || '/agents/') > 0
), profile_id);
UPDATE agent_heartbeat_snapshots
SET profile_id = COALESCE((
  SELECT id FROM profiles
  WHERE instr('/' || replace(agent_heartbeat_snapshots.source_path, char(92), '/'), '/profiles/' || profiles.name || '/agents/') > 0
), profile_id);
UPDATE agent_heartbeat_revisions
SET profile_id = COALESCE((
  SELECT id FROM profiles
  WHERE instr('/' || replace(agent_heartbeat_revisions.source_path, char(92), '/'), '/profiles/' || profiles.name || '/agents/') > 0
), profile_id);
