CREATE TABLE extension_dev_links (
		extension_name TEXT NOT NULL,
		workspace_id TEXT NOT NULL,
		origin_path TEXT NOT NULL,
		bundle_generation TEXT NOT NULL,
		linked_at TIMESTAMP NOT NULL,
		network_requirement_digest TEXT NOT NULL DEFAULT '',
		network_confirmed_by TEXT,
		network_confirmed_at TEXT,
		UNIQUE (extension_name, workspace_id)
	);
