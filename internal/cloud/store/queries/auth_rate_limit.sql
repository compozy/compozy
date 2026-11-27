-- SPDX-License-Identifier: FSL-1.1-MIT

-- name: ChargeAuthIPWindow :execrows
INSERT INTO auth_ip_rate_windows AS w (scope, client_ip_sha256, expires_at, requests)
VALUES ($1, $2, $3, 1)
ON CONFLICT (scope, client_ip_sha256, expires_at) DO UPDATE SET requests = w.requests + 1
WHERE w.requests < sqlc.arg(request_limit)::integer;

-- name: SweepAuthIPWindows :exec
DELETE FROM auth_ip_rate_windows WHERE expires_at <= $1;
