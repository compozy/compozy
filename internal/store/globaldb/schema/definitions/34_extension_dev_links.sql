CREATE TABLE extension_dev_links (
		extension_name TEXT NOT NULL,
		workspace_id TEXT NOT NULL,
		origin_path TEXT NOT NULL,
		bundle_generation TEXT NOT NULL,
		linked_at TIMESTAMP NOT NULL,
		format TEXT NOT NULL DEFAULT 'compozy',
		ingest_diagnostics_json TEXT NOT NULL DEFAULT '[]',
		network_requirement_digest TEXT NOT NULL DEFAULT '',
		network_confirmed_by TEXT,
		network_confirmed_at TEXT,
		UNIQUE (extension_name, workspace_id)
	);

CREATE TRIGGER extension_dev_links_profile_enablement_insert
BEFORE INSERT ON extension_profile_enablement
WHEN NOT EXISTS (
		SELECT 1 FROM extensions WHERE name = NEW.extension_name
	)
	AND NOT EXISTS (
		SELECT 1 FROM extension_dev_links WHERE extension_name = NEW.extension_name
	)
BEGIN
	SELECT RAISE(ABORT, 'extension_not_found');
END;

CREATE TRIGGER extension_dev_links_profile_enablement_delete
AFTER DELETE ON extension_dev_links
WHEN NOT EXISTS (
		SELECT 1 FROM extensions WHERE name = OLD.extension_name
	)
	AND NOT EXISTS (
		SELECT 1 FROM extension_dev_links WHERE extension_name = OLD.extension_name
	)
BEGIN
	DELETE FROM extension_profile_enablement WHERE extension_name = OLD.extension_name;
END;
