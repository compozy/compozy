-- +goose Up
CREATE TABLE ingress_endpoint_state (
	id INTEGER NOT NULL PRIMARY KEY CHECK (id = 1),
	endpoint_url TEXT NOT NULL,
	endpoint_generation INTEGER NOT NULL CHECK (endpoint_generation > 0),
	reachable BOOLEAN NOT NULL DEFAULT 0,
	source TEXT NOT NULL CHECK (source IN ('gateway', 'remote')),
	updated_at TEXT NOT NULL
);

INSERT INTO ingress_endpoint_state (id, endpoint_url, endpoint_generation, reachable, source, updated_at)
SELECT 1, endpoint_url, endpoint_generation, reachable, 'gateway', updated_at
FROM gateway_endpoint_state
WHERE tier = 'public';

-- +goose Down
DROP TABLE ingress_endpoint_state;
