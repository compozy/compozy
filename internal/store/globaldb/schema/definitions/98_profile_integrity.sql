CREATE TRIGGER profile_selections_workspace_delete AFTER DELETE ON workspaces BEGIN
	DELETE FROM profile_selections WHERE lens = 'workspace' AND workspace_id = OLD.id;
END;

CREATE TRIGGER cmd_palette_usage_profile_lens_insert BEFORE INSERT ON cmd_palette_usage
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;
CREATE TRIGGER cmd_palette_usage_profile_lens_update BEFORE UPDATE OF profile_lens_id ON cmd_palette_usage
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;
CREATE TRIGGER cmd_palette_query_hits_profile_lens_insert BEFORE INSERT ON cmd_palette_query_hits
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;
CREATE TRIGGER cmd_palette_query_hits_profile_lens_update BEFORE UPDATE OF profile_lens_id ON cmd_palette_query_hits
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;
CREATE TRIGGER cmd_palette_pins_profile_lens_insert BEFORE INSERT ON cmd_palette_pins
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;
CREATE TRIGGER cmd_palette_pins_profile_lens_update BEFORE UPDATE OF profile_lens_id ON cmd_palette_pins
WHEN NEW.profile_lens_id <> '@all' AND NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_lens_id)
BEGIN SELECT RAISE(ABORT, 'profile_lens_not_found'); END;

CREATE TRIGGER profiles_palette_cleanup AFTER DELETE ON profiles BEGIN
	DELETE FROM cmd_palette_usage WHERE profile_lens_id = OLD.id;
	DELETE FROM cmd_palette_query_hits WHERE profile_lens_id = OLD.id;
	DELETE FROM cmd_palette_pins WHERE profile_lens_id = OLD.id;
END;

CREATE TRIGGER sessions_profile_owner_immutable BEFORE UPDATE OF profile_id ON sessions
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER sessions_profile_owner_active BEFORE INSERT ON sessions BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER tasks_profile_owner_immutable BEFORE UPDATE OF profile_id ON tasks
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER tasks_profile_owner_active BEFORE INSERT ON tasks BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER loop_runs_profile_owner_immutable BEFORE UPDATE OF profile_id ON loop_runs
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER loop_runs_profile_owner_active BEFORE INSERT ON loop_runs BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER automation_jobs_profile_owner_immutable BEFORE UPDATE OF profile_id ON automation_jobs
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER automation_jobs_profile_owner_active BEFORE INSERT ON automation_jobs BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER automation_triggers_profile_owner_immutable BEFORE UPDATE OF profile_id ON automation_triggers
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER automation_triggers_profile_owner_active BEFORE INSERT ON automation_triggers BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER automation_suggestions_profile_owner_immutable BEFORE UPDATE OF profile_id ON automation_suggestions
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER automation_suggestions_profile_owner_active BEFORE INSERT ON automation_suggestions BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER bridge_instances_profile_owner_immutable BEFORE UPDATE OF profile_id ON bridge_instances
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER bridge_instances_profile_owner_active BEFORE INSERT ON bridge_instances BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER worktrees_profile_owner_immutable BEFORE UPDATE OF profile_id ON worktrees
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER worktrees_profile_owner_active BEFORE INSERT ON worktrees BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER network_channels_profile_owner_immutable BEFORE UPDATE OF profile_id ON network_channels
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER network_channels_profile_owner_active BEFORE INSERT ON network_channels BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER network_direct_rooms_profile_owner_immutable BEFORE UPDATE OF profile_id ON network_direct_rooms
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER network_direct_rooms_profile_owner_active BEFORE INSERT ON network_direct_rooms BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER network_threads_profile_owner_immutable BEFORE UPDATE OF profile_id ON network_threads
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER network_threads_profile_owner_active BEFORE INSERT ON network_threads BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER network_work_profile_owner_immutable BEFORE UPDATE OF profile_id ON network_work
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER network_work_profile_owner_active BEFORE INSERT ON network_work BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER notification_cursors_profile_owner_immutable BEFORE UPDATE OF profile_id ON notification_cursors
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER notification_cursors_profile_owner_active BEFORE INSERT ON notification_cursors BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER tool_approval_grants_profile_owner_immutable BEFORE UPDATE OF profile_id ON tool_approval_grants
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER tool_approval_grants_profile_owner_active BEFORE INSERT ON tool_approval_grants BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER event_summaries_profile_owner_immutable BEFORE UPDATE OF profile_id ON event_summaries
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER event_summaries_profile_owner_active BEFORE INSERT ON event_summaries BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER dead_entities_profile_owner_immutable BEFORE UPDATE OF profile_id ON dead_entities
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER dead_entities_profile_owner_active BEFORE INSERT ON dead_entities BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER token_usage_daily_profile_owner_immutable BEFORE UPDATE OF profile_id ON token_usage_daily
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER token_usage_daily_profile_owner_active BEFORE INSERT ON token_usage_daily BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;

CREATE TRIGGER tool_approval_pending_profile_owner_immutable BEFORE UPDATE OF profile_id ON tool_approval_pending
WHEN NEW.profile_id <> OLD.profile_id BEGIN SELECT RAISE(ABORT, 'profile_owner_immutable'); END;
CREATE TRIGGER tool_approval_pending_profile_owner_active BEFORE INSERT ON tool_approval_pending BEGIN
	SELECT CASE WHEN NOT EXISTS (SELECT 1 FROM profiles WHERE id = NEW.profile_id AND state = 'active') THEN RAISE(ABORT, 'profile_archived') END;
	SELECT CASE WHEN EXISTS (SELECT 1 FROM profile_lifecycle_ops WHERE profile_id = NEW.profile_id AND status <> 'done') THEN RAISE(ABORT, 'profile_unavailable') END;
END;
