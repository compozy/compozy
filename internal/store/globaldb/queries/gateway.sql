-- name: UpsertGatewayProvider :exec
INSERT INTO gateway_providers (name, install_source, digest_confirmed, confirmed_at)
VALUES (
  sqlc.arg(name), sqlc.arg(install_source), sqlc.narg(digest_confirmed), sqlc.narg(confirmed_at)
)
ON CONFLICT(name) DO UPDATE SET
  install_source = excluded.install_source,
  digest_confirmed = excluded.digest_confirmed,
  confirmed_at = excluded.confirmed_at;

-- name: GetGatewayProvider :one
SELECT name, install_source, digest_confirmed, confirmed_at
FROM gateway_providers
WHERE name = sqlc.arg(name);

-- name: TransitionGatewayProviderActivation :one
INSERT INTO gateway_provider_activations (
  provider_name, tier, desired_state, observed_state, generation, last_health_at, last_error
)
VALUES (
  sqlc.arg(provider_name), sqlc.arg(tier), sqlc.arg(desired_state),
  'down', CAST(sqlc.arg(expected_generation) AS INTEGER) + 1, NULL, NULL
)
ON CONFLICT(provider_name, tier) DO UPDATE SET
  desired_state = excluded.desired_state,
  observed_state = CASE
    WHEN excluded.desired_state = 'disabled' THEN 'down'
    ELSE gateway_provider_activations.observed_state
  END,
  generation = gateway_provider_activations.generation + 1,
  last_health_at = CASE
    WHEN excluded.desired_state = 'disabled' THEN NULL
    ELSE gateway_provider_activations.last_health_at
  END,
  last_error = CASE
    WHEN excluded.desired_state = 'disabled' THEN NULL
    ELSE gateway_provider_activations.last_error
  END
WHERE gateway_provider_activations.generation = excluded.generation - 1
RETURNING provider_name, tier, desired_state, observed_state, generation, last_health_at, last_error;

-- name: TransitionGatewaySurfaceExposure :one
INSERT INTO gateway_surface_exposure (
  surface, tier, desired_state, observed_state, generation, consented_at
)
VALUES (
  sqlc.arg(surface), sqlc.arg(tier), sqlc.arg(desired_state),
  'off', CAST(sqlc.arg(expected_generation) AS INTEGER) + 1, sqlc.narg(consented_at)
)
ON CONFLICT(surface, tier) DO UPDATE SET
  desired_state = excluded.desired_state,
  observed_state = CASE
    WHEN excluded.desired_state = 'disabled' THEN 'off'
    ELSE gateway_surface_exposure.observed_state
  END,
  generation = gateway_surface_exposure.generation + 1,
  consented_at = CASE
    WHEN excluded.desired_state = 'disabled' THEN NULL
    ELSE excluded.consented_at
  END
WHERE gateway_surface_exposure.generation = excluded.generation - 1
RETURNING surface, tier, desired_state, observed_state, generation, consented_at;

-- name: UpdateGatewayProviderObserved :execrows
UPDATE gateway_provider_activations
SET observed_state = sqlc.arg(observed_state),
    last_health_at = sqlc.narg(last_health_at),
    last_error = sqlc.narg(last_error)
WHERE provider_name = sqlc.arg(provider_name)
  AND tier = sqlc.arg(tier)
  AND generation = sqlc.arg(expected_generation);

-- name: UpdateGatewaySurfaceObserved :execrows
UPDATE gateway_surface_exposure
SET observed_state = sqlc.arg(observed_state)
WHERE surface = sqlc.arg(surface)
  AND tier = sqlc.arg(tier)
  AND generation = sqlc.arg(expected_generation);

-- name: GetGatewayProviderActivation :one
SELECT provider_name, tier, desired_state, observed_state, generation, last_health_at, last_error
FROM gateway_provider_activations
WHERE provider_name = sqlc.arg(provider_name) AND tier = sqlc.arg(tier);

-- name: GetGatewaySurfaceExposure :one
SELECT surface, tier, desired_state, observed_state, generation, consented_at
FROM gateway_surface_exposure
WHERE surface = sqlc.arg(surface) AND tier = sqlc.arg(tier);

-- name: ListGatewayProviderActivations :many
SELECT provider_name, tier, desired_state, observed_state, generation, last_health_at, last_error
FROM gateway_provider_activations
ORDER BY tier, provider_name;

-- name: ListGatewaySurfaceExposure :many
SELECT surface, tier, desired_state, observed_state, generation, consented_at
FROM gateway_surface_exposure
ORDER BY tier, surface;

-- name: DisableAllGatewayProviderActivations :execrows
UPDATE gateway_provider_activations
SET desired_state = 'disabled',
    observed_state = 'down',
    generation = generation + 1,
    last_health_at = NULL,
    last_error = NULL
WHERE desired_state <> 'disabled' OR observed_state <> 'down';

-- name: DisableAllGatewaySurfaceExposure :execrows
UPDATE gateway_surface_exposure
SET desired_state = 'disabled',
    observed_state = 'off',
    generation = generation + 1,
    consented_at = NULL
WHERE desired_state <> 'disabled' OR observed_state <> 'off' OR consented_at IS NOT NULL;
