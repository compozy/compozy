-- Tombstones preserve logical deletion across the independent telemetry WAL.
CREATE TABLE telemetry_deleted_scopes (
    kind TEXT NOT NULL CHECK (kind IN ('profile', 'workspace')),
    scope_id TEXT NOT NULL,
    PRIMARY KEY (kind, scope_id)
);

CREATE TRIGGER telemetry_profile_deleted AFTER DELETE ON profiles BEGIN
    INSERT OR IGNORE INTO telemetry_deleted_scopes(kind, scope_id) VALUES ('profile', OLD.id);
END;
