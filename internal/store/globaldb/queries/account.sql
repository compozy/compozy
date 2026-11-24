-- name: ReadInstallationAccount :one
SELECT * FROM installation_account WHERE id = 1;

-- name: SaveInstallationAccount :execrows
UPDATE installation_account SET
  state = sqlc.arg(state), source = sqlc.narg(source), account_id = sqlc.narg(account_id),
  email = sqlc.narg(email), name = sqlc.narg(name), provider = sqlc.narg(provider),
  device_id = sqlc.narg(device_id), device_name = sqlc.narg(device_name), device_admin = sqlc.narg(device_admin),
  marketing_opt_in = sqlc.arg(marketing_opt_in), plan_json = sqlc.narg(plan_json),
  signed_out_reason = sqlc.narg(signed_out_reason), auth_generation = sqlc.arg(auth_generation),
  last_validated_at = sqlc.narg(last_validated_at),
  gate_policy_jws = sqlc.narg(gate_policy_jws),
  gate_policy_keys = sqlc.narg(gate_policy_keys),
  gate_policy_fetched_at = sqlc.narg(gate_policy_fetched_at),
  offline_grace_started_at = sqlc.narg(offline_grace_started_at),
  token_name = sqlc.narg(token_name),
  sign_in_notices_dismissed_at = sqlc.narg(sign_in_notices_dismissed_at)
WHERE id = 1 AND auth_generation = sqlc.arg(previous_generation);
