CREATE TABLE gateway_device_sessions (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL CHECK (length(trim(name)) > 0),
	token_hash TEXT NOT NULL CHECK (length(token_hash) = 64),
	actor_kind TEXT NOT NULL CHECK (actor_kind IN ('operator_device', 'cli_profile')),
	pairing_origin TEXT NOT NULL CHECK (length(trim(pairing_origin)) > 0),
	revoke_epoch INTEGER NOT NULL DEFAULT 0 CHECK (revoke_epoch >= 0),
	created_at TEXT NOT NULL,
	last_seen_at TEXT,
	revoked_at TEXT
);

CREATE UNIQUE INDEX gateway_device_sessions_token_hash
	ON gateway_device_sessions(token_hash);

CREATE TABLE gateway_providers (
	name TEXT PRIMARY KEY CHECK (length(trim(name)) > 0),
	install_source TEXT NOT NULL CHECK (length(trim(install_source)) > 0),
	digest_confirmed TEXT,
	confirmed_at TEXT
);

CREATE TABLE gateway_provider_activations (
	provider_name TEXT NOT NULL REFERENCES gateway_providers(name) ON DELETE CASCADE,
	tier TEXT NOT NULL CHECK (tier IN ('private', 'public')),
	desired_state TEXT NOT NULL CHECK (desired_state IN ('enabled', 'disabled')),
	observed_state TEXT NOT NULL CHECK (observed_state IN ('down', 'establishing', 'up', 'degraded')),
	generation INTEGER NOT NULL DEFAULT 0 CHECK (generation >= 0),
	last_health_at TEXT,
	last_error TEXT,
	PRIMARY KEY (provider_name, tier)
);

CREATE UNIQUE INDEX gateway_active_provider_per_tier
	ON gateway_provider_activations(tier)
	WHERE desired_state = 'enabled';

CREATE TABLE gateway_surface_exposure (
	surface TEXT NOT NULL CHECK (surface IN ('operator_ui', 'webhook_ingress')),
	tier TEXT NOT NULL CHECK (tier IN ('private', 'public')),
	desired_state TEXT NOT NULL CHECK (desired_state IN ('enabled', 'disabled')),
	observed_state TEXT NOT NULL CHECK (observed_state IN ('off', 'on')),
	generation INTEGER NOT NULL DEFAULT 0 CHECK (generation >= 0),
	consented_at TEXT,
	PRIMARY KEY (surface, tier),
	CHECK (surface <> 'operator_ui' OR tier <> 'public' OR desired_state = 'disabled' OR consented_at IS NOT NULL)
);
