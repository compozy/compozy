-- +goose Up
-- create "event_summary_outbox" table
CREATE TABLE `event_summary_outbox` (`seq` integer NULL PRIMARY KEY AUTOINCREMENT, `id` text NOT NULL, `profile_id` text NOT NULL, `session_id` text NOT NULL DEFAULT '', `workspace_id` text NOT NULL DEFAULT '', `worktree_id` text NOT NULL DEFAULT '', `type` text NOT NULL, `agent_name` text NOT NULL DEFAULT '', `content_json` text NOT NULL DEFAULT '', `task_id` text NOT NULL DEFAULT '', `run_id` text NOT NULL DEFAULT '', `workflow_id` text NOT NULL DEFAULT '', `claim_token_hash` text NOT NULL DEFAULT '', `lease_until` text NOT NULL DEFAULT '', `coordinator_session_id` text NOT NULL DEFAULT '', `scheduler_reason` text NOT NULL DEFAULT '', `hook_event` text NOT NULL DEFAULT '', `hook_name` text NOT NULL DEFAULT '', `actor_kind` text NOT NULL DEFAULT '', `actor_id` text NOT NULL DEFAULT '', `release_reason` text NOT NULL DEFAULT '', `parent_session_id` text NOT NULL DEFAULT '', `root_session_id` text NOT NULL DEFAULT '', `spawn_depth` integer NOT NULL DEFAULT 0, `summary` text NULL, `timestamp` text NOT NULL, `provider` text NOT NULL DEFAULT '', `outcome` text NOT NULL DEFAULT 'info', CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE);
-- create index "event_summary_outbox_id" to table: "event_summary_outbox"
CREATE UNIQUE INDEX `event_summary_outbox_id` ON `event_summary_outbox` (`id`);
-- apply declarative trigger "event_summary_outbox_profile_owner_active" on table "event_summary_outbox"
-- +goose StatementBegin
CREATE TRIGGER event_summary_outbox_profile_owner_active BEFORE INSERT ON event_summary_outbox BEGIN
 SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
 SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
-- +goose StatementEnd
