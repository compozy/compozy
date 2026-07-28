-- name: DeleteTaskExecutionProfile :execrows
DELETE FROM task_execution_profiles WHERE task_id = sqlc.arg(task_id);

-- name: GetTaskExecutionProfile :one
SELECT task_id, coordinator_mode, coordinator_agent_name, coordinator_provider,
       coordinator_model, coordinator_guidance, worker_mode, worker_agent_name,
       worker_provider, worker_model, review_agent_name, review_provider,
       review_model, sandbox_mode, sandbox_ref, runtime_mode, created_at, updated_at,
       network_mode, network_channel_strategy, network_channel, network_bounds_json
FROM task_execution_profiles
WHERE task_id = sqlc.arg(task_id);

-- name: UpsertTaskExecutionProfile :exec
INSERT INTO task_execution_profiles (
  task_id, coordinator_mode, coordinator_agent_name, coordinator_provider,
  coordinator_model, coordinator_guidance, worker_mode, worker_agent_name,
  worker_provider, worker_model, review_agent_name, review_provider,
  review_model, sandbox_mode, sandbox_ref, runtime_mode, created_at, updated_at,
  network_mode, network_channel_strategy, network_channel, network_bounds_json
) VALUES (
  sqlc.arg(task_id), sqlc.arg(coordinator_mode), sqlc.arg(coordinator_agent_name),
  sqlc.arg(coordinator_provider), sqlc.arg(coordinator_model), sqlc.arg(coordinator_guidance),
  sqlc.arg(worker_mode), sqlc.arg(worker_agent_name), sqlc.arg(worker_provider),
  sqlc.arg(worker_model), sqlc.arg(review_agent_name), sqlc.arg(review_provider),
  sqlc.arg(review_model), sqlc.arg(sandbox_mode), sqlc.arg(sandbox_ref),
  sqlc.arg(runtime_mode), sqlc.arg(created_at), sqlc.arg(updated_at),
  sqlc.arg(network_mode), sqlc.arg(network_channel_strategy), sqlc.arg(network_channel),
  sqlc.arg(network_bounds_json)
)
ON CONFLICT(task_id) DO UPDATE SET
  coordinator_mode = excluded.coordinator_mode,
  coordinator_agent_name = excluded.coordinator_agent_name,
  coordinator_provider = excluded.coordinator_provider,
  coordinator_model = excluded.coordinator_model,
  coordinator_guidance = excluded.coordinator_guidance,
  worker_mode = excluded.worker_mode,
  worker_agent_name = excluded.worker_agent_name,
  worker_provider = excluded.worker_provider,
  worker_model = excluded.worker_model,
  review_agent_name = excluded.review_agent_name,
  review_provider = excluded.review_provider,
  review_model = excluded.review_model,
  sandbox_mode = excluded.sandbox_mode,
  sandbox_ref = excluded.sandbox_ref,
  runtime_mode = excluded.runtime_mode,
  network_mode = excluded.network_mode,
  network_channel_strategy = excluded.network_channel_strategy,
  network_channel = excluded.network_channel,
  network_bounds_json = excluded.network_bounds_json,
  updated_at = excluded.updated_at;

-- name: DeleteTaskProfileAgents :exec
DELETE FROM task_profile_agents WHERE task_id = sqlc.arg(task_id);

-- name: DeleteTaskProfileChannels :exec
DELETE FROM task_profile_channels WHERE task_id = sqlc.arg(task_id);

-- name: DeleteTaskProfilePeers :exec
DELETE FROM task_profile_peers WHERE task_id = sqlc.arg(task_id);

-- name: DeleteTaskProfileCapabilities :exec
DELETE FROM task_profile_capabilities WHERE task_id = sqlc.arg(task_id);

-- name: InsertTaskProfileAgent :exec
INSERT INTO task_profile_agents (task_id, role, preference, agent_name)
VALUES (sqlc.arg(task_id), sqlc.arg(role), sqlc.arg(preference), sqlc.arg(value));

-- name: InsertTaskProfileChannel :exec
INSERT INTO task_profile_channels (task_id, role, preference, channel_id)
VALUES (sqlc.arg(task_id), sqlc.arg(role), sqlc.arg(preference), sqlc.arg(value));

-- name: InsertTaskProfilePeer :exec
INSERT INTO task_profile_peers (task_id, role, preference, peer_id)
VALUES (sqlc.arg(task_id), sqlc.arg(role), sqlc.arg(preference), sqlc.arg(value));

-- name: InsertTaskProfileCapability :exec
INSERT INTO task_profile_capabilities (task_id, role, preference, capability_id)
VALUES (sqlc.arg(task_id), sqlc.arg(role), sqlc.arg(preference), sqlc.arg(value));

-- name: ListTaskProfileAgents :many
SELECT role, preference, agent_name AS value
FROM task_profile_agents
WHERE task_id = sqlc.arg(task_id)
ORDER BY role ASC, preference ASC, agent_name ASC;

-- name: ListTaskProfileChannels :many
SELECT role, preference, channel_id AS value
FROM task_profile_channels
WHERE task_id = sqlc.arg(task_id)
ORDER BY role ASC, preference ASC, channel_id ASC;

-- name: ListTaskProfilePeers :many
SELECT role, preference, peer_id AS value
FROM task_profile_peers
WHERE task_id = sqlc.arg(task_id)
ORDER BY role ASC, preference ASC, peer_id ASC;

-- name: ListTaskProfileCapabilities :many
SELECT role, preference, capability_id AS value
FROM task_profile_capabilities
WHERE task_id = sqlc.arg(task_id)
ORDER BY role ASC, preference ASC, capability_id ASC;
