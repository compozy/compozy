-- +goose Up
-- create "deleted_session_budget_lineage" table
CREATE TABLE `deleted_session_budget_lineage` (`id` text NOT NULL, `profile_id` text NOT NULL, `parent_session_id` text NULL, `instance_id` text NULL, `lineage_kind` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE);
-- create index "idx_deleted_session_budget_lineage_parent" to table: "deleted_session_budget_lineage"
CREATE INDEX `idx_deleted_session_budget_lineage_parent` ON `deleted_session_budget_lineage` (`parent_session_id`, `profile_id`);
-- apply declarative trigger "preserve_deleted_session_budget_lineage" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER preserve_deleted_session_budget_lineage
BEFORE DELETE ON sessions
WHEN EXISTS (
    SELECT 1 FROM sessions child WHERE child.parent_session_id = OLD.id
      AND child.profile_id = OLD.profile_id
      AND child.lineage_kind NOT IN ('continue', 'fork', 'provenance')
)
OR EXISTS (
    SELECT 1 FROM deleted_session_budget_lineage child WHERE child.parent_session_id = OLD.id
      AND child.profile_id = OLD.profile_id
      AND child.lineage_kind NOT IN ('continue', 'fork', 'provenance')
)
BEGIN
    INSERT INTO deleted_session_budget_lineage(id, profile_id, parent_session_id, instance_id, lineage_kind)
    VALUES (OLD.id, OLD.profile_id, OLD.parent_session_id, OLD.instance_id, OLD.lineage_kind)
    ON CONFLICT(id) DO UPDATE SET profile_id = excluded.profile_id,
        parent_session_id = excluded.parent_session_id, instance_id = excluded.instance_id,
        lineage_kind = excluded.lineage_kind;
END;
-- +goose StatementEnd
