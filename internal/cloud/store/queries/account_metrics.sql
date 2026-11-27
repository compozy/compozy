-- SPDX-License-Identifier: FSL-1.1-MIT

-- name: CloudAccountTotals :many
SELECT first_sign_in_provider AS provider, count(*) AS total
FROM accounts GROUP BY first_sign_in_provider ORDER BY first_sign_in_provider;

-- name: CloudActiveCounts :one
SELECT
    (SELECT count(*) FROM accounts WHERE last_seen_at >= sqlc.arg(since)::timestamptz)::bigint AS accounts,
    (SELECT count(DISTINCT installation_id) FROM devices WHERE last_seen_at >= sqlc.arg(since))::bigint AS sessions,
    (SELECT count(DISTINCT coalesce(installation_id, id)) FROM computers
     WHERE greatest(last_control_at, linked_at) >= sqlc.arg(since))::bigint AS links,
    lifecycle_token_installations(sqlc.arg(since)::timestamptz)::bigint AS tokens;
