CREATE TABLE ingress_endpoint_state (
	id INTEGER NOT NULL PRIMARY KEY CHECK (id = 1),
	endpoint_url TEXT NOT NULL,
	endpoint_generation INTEGER NOT NULL CHECK (endpoint_generation > 0),
	reachable BOOLEAN NOT NULL DEFAULT 0,
	source TEXT NOT NULL CHECK (source IN ('gateway', 'remote')),
	updated_at TEXT NOT NULL
);
