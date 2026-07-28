CREATE TABLE task_execution_profiles (
			task_id                  TEXT PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
			coordinator_mode         TEXT NOT NULL DEFAULT 'inherit' CHECK (
				coordinator_mode IN ('inherit', 'guided')
			),
			coordinator_agent_name   TEXT NOT NULL DEFAULT '',
			coordinator_provider     TEXT NOT NULL DEFAULT '',
			coordinator_model        TEXT NOT NULL DEFAULT '',
			coordinator_guidance     TEXT NOT NULL DEFAULT '',
			worker_mode              TEXT NOT NULL DEFAULT 'inherit' CHECK (
				worker_mode IN ('inherit', 'select')
			),
			worker_agent_name        TEXT NOT NULL DEFAULT '',
			worker_provider          TEXT NOT NULL DEFAULT '',
			worker_model             TEXT NOT NULL DEFAULT '',
			review_agent_name        TEXT NOT NULL DEFAULT '',
			review_provider          TEXT NOT NULL DEFAULT '',
			review_model             TEXT NOT NULL DEFAULT '',
			sandbox_mode             TEXT NOT NULL DEFAULT 'inherit' CHECK (
				sandbox_mode IN ('inherit', 'none', 'ref')
			),
			sandbox_ref              TEXT NOT NULL DEFAULT '',
			created_at               TEXT NOT NULL,
			updated_at               TEXT NOT NULL,
			runtime_mode             TEXT NOT NULL DEFAULT 'default' CHECK (
				runtime_mode IN ('default', 'evidence')
			),
			network_mode             TEXT NOT NULL DEFAULT '' CHECK (network_mode IN ('', 'local', 'live')),
			network_channel_strategy TEXT CHECK (
				network_channel_strategy IS NULL OR network_channel_strategy IN ('named', 'run', 'loop_run')
			),
			network_channel          TEXT,
			network_bounds_json      TEXT CHECK (
				network_bounds_json IS NULL OR json_valid(network_bounds_json)
			),
			CHECK (
				(sandbox_mode = 'ref' AND sandbox_ref <> '') OR
				(sandbox_mode <> 'ref' AND sandbox_ref = '')
			)
		);

CREATE TABLE task_profile_agents (
			task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			role        TEXT NOT NULL CHECK (role IN ('worker', 'review', 'participant')),
			preference  TEXT NOT NULL CHECK (preference IN ('required', 'allowed', 'preferred')),
			agent_name  TEXT NOT NULL CHECK (agent_name <> ''),
			PRIMARY KEY (task_id, role, preference, agent_name)
		);

CREATE TABLE task_profile_capabilities (
			task_id       TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			role          TEXT NOT NULL CHECK (role IN ('worker', 'review', 'participant')),
			preference    TEXT NOT NULL CHECK (preference IN ('required', 'preferred')),
			capability_id TEXT NOT NULL CHECK (capability_id <> ''),
			PRIMARY KEY (task_id, role, preference, capability_id)
		);

CREATE TABLE task_profile_channels (
			task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			role        TEXT NOT NULL CHECK (role IN ('review', 'participant')),
			preference  TEXT NOT NULL CHECK (preference IN ('allowed', 'preferred')),
			channel_id  TEXT NOT NULL CHECK (channel_id <> ''),
			PRIMARY KEY (task_id, role, preference, channel_id)
		);

CREATE TABLE task_profile_peers (
			task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			role        TEXT NOT NULL CHECK (role IN ('review', 'participant')),
			preference  TEXT NOT NULL CHECK (preference IN ('allowed', 'preferred')),
			peer_id     TEXT NOT NULL CHECK (peer_id <> ''),
			PRIMARY KEY (task_id, role, preference, peer_id)
		);

CREATE INDEX task_execution_profiles_task_id_idx
			ON task_execution_profiles(task_id);

CREATE INDEX task_profile_agents_lookup_idx
			ON task_profile_agents(role, preference, agent_name, task_id);

CREATE INDEX task_profile_capabilities_lookup_idx
			ON task_profile_capabilities(role, preference, capability_id, task_id);

CREATE INDEX task_profile_channels_lookup_idx
			ON task_profile_channels(role, preference, channel_id, task_id);

CREATE INDEX task_profile_peers_lookup_idx
			ON task_profile_peers(role, preference, peer_id, task_id);
