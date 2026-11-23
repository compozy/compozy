-- name: InsertDeviceGrant :execrows
INSERT INTO device_grants (
    device_code_sha256, user_code, provider, client_name, client_platform, installation_id, request_location, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (user_code) DO NOTHING;

-- name: FindDeviceGrantByCode :one
SELECT * FROM device_grants WHERE user_code = $1;

-- name: LockDeviceGrant :one
SELECT * FROM device_grants WHERE device_code_sha256 = $1 FOR UPDATE;

-- name: BindDeviceGrantOAuth :execrows
UPDATE device_grants
SET oauth_state_sha256 = $2, pkce_verifier_enc = $3, activation_nonce_sha256 = $4
WHERE device_code_sha256 = $1 AND state = 'pending' AND expires_at > sqlc.arg(as_of);

-- name: InsertCloudOAuthAttempt :exec
INSERT INTO cloud_oauth_attempts (
    state_sha256, device_code_sha256, browser_request, provider, cookie_sha256,
    pkce_verifier_enc, nonce_sha256, max_age_zero, created_at, expires_at
) VALUES ($1, $2, sqlc.narg(browser_request)::text::jsonb, $3, $4, $5, $6, $7, $8, $9);

-- name: ConsumeCloudOAuthAttempt :one
UPDATE cloud_oauth_attempts SET consumed_at = sqlc.arg(as_of)
WHERE state_sha256 = $1 AND cookie_sha256 = $2 AND provider = $3
  AND consumed_at IS NULL AND expires_at > sqlc.arg(as_of)
RETURNING *;

-- name: AuthorizeDeviceGrant :execrows
UPDATE device_grants SET state = 'authorized', account_id = $2, provider = $3,
    new_account = $4, max_age_zero = $5,
    oauth_state_sha256 = NULL, pkce_verifier_enc = NULL, activation_nonce_sha256 = NULL
WHERE device_code_sha256 = $1 AND state = 'pending' AND expires_at > sqlc.arg(as_of);

-- name: CancelDeviceGrant :exec
UPDATE device_grants SET state = 'cancelled', failure_code = $2,
    oauth_state_sha256 = NULL, pkce_verifier_enc = NULL, activation_nonce_sha256 = NULL
WHERE device_code_sha256 = $1
  AND ((state = 'pending') OR (state = 'authorized' AND sqlc.arg(allow_authorized)::boolean))
  AND expires_at > sqlc.arg(as_of);

-- name: ExpireDeviceGrant :exec
UPDATE device_grants SET state = 'expired',
    oauth_state_sha256 = NULL, pkce_verifier_enc = NULL, activation_nonce_sha256 = NULL
WHERE device_code_sha256 = $1 AND state IN ('pending', 'authorized');

-- name: RecordDeviceGrantPoll :exec
UPDATE device_grants
SET last_poll_at = sqlc.arg(as_of), poll_interval_seconds = sqlc.arg(interval_seconds)
WHERE device_code_sha256 = $1;

-- name: RedeemDeviceGrant :execrows
UPDATE device_grants SET state = 'redeemed'
WHERE device_code_sha256 = $1 AND state = 'authorized' AND expires_at > sqlc.arg(as_of);

-- name: InsertEnrollmentToken :exec
INSERT INTO enrollment_tokens (token_sha256, account_id, device_code_sha256, provider, max_age_zero, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: FindEnrollmentToken :one
SELECT e.*, g.installation_id, g.request_location
FROM enrollment_tokens e JOIN device_grants g USING (device_code_sha256)
WHERE e.token_sha256 = $1 AND e.used_at IS NULL AND e.expires_at > sqlc.arg(as_of);

-- name: ConsumeEnrollmentToken :execrows
UPDATE enrollment_tokens SET used_at = sqlc.arg(as_of)
WHERE token_sha256 = $1 AND used_at IS NULL AND expires_at > sqlc.arg(as_of);

-- name: SweepAuthAttempts :exec
DELETE FROM cloud_oauth_attempts WHERE expires_at <= $1;

-- name: SweepExpiredDeviceGrants :exec
UPDATE device_grants SET state = 'expired',
    oauth_state_sha256 = NULL, pkce_verifier_enc = NULL, activation_nonce_sha256 = NULL
WHERE expires_at <= $1 AND state IN ('pending', 'authorized');
