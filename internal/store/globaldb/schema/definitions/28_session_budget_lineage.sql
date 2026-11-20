-- A plain delete may leave a child session alive. Keep the deleted edge
-- available for receipt attribution without restoring its catalog entry.
CREATE TABLE deleted_session_budget_lineage (
    id TEXT NOT NULL PRIMARY KEY,
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    parent_session_id TEXT,
    instance_id TEXT,
    lineage_kind TEXT NOT NULL
);

CREATE INDEX idx_deleted_session_budget_lineage_parent
    ON deleted_session_budget_lineage(parent_session_id, profile_id);

CREATE TRIGGER preserve_deleted_session_budget_lineage
BEFORE DELETE ON sessions
WHEN EXISTS (
    SELECT 1 FROM sessions child WHERE child.parent_session_id = OLD.id
      AND child.profile_id = OLD.profile_id
      AND child.lineage_kind NOT IN ('continue', 'fork', 'provenance', 'side')
)
OR EXISTS (
    SELECT 1 FROM deleted_session_budget_lineage child WHERE child.parent_session_id = OLD.id
      AND child.profile_id = OLD.profile_id
      AND child.lineage_kind NOT IN ('continue', 'fork', 'provenance', 'side')
)
BEGIN
    INSERT INTO deleted_session_budget_lineage(id, profile_id, parent_session_id, instance_id, lineage_kind)
    VALUES (OLD.id, OLD.profile_id, OLD.parent_session_id, OLD.instance_id, OLD.lineage_kind)
    ON CONFLICT(id) DO UPDATE SET profile_id = excluded.profile_id,
        parent_session_id = excluded.parent_session_id, instance_id = excluded.instance_id,
        lineage_kind = excluded.lineage_kind;
END;
