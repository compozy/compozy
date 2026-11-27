-- SPDX-License-Identifier: FSL-1.1-MIT
-- name: RecordInstallationActivity :exec
WITH observed AS (
    INSERT INTO cloud_installations_seen (account_id, installation_id, first_seen_at, last_seen_at)
    VALUES (sqlc.arg(account_id)::text, sqlc.arg(installation_id)::text,
        sqlc.arg(seen_at)::timestamptz, sqlc.arg(seen_at)::timestamptz)
    ON CONFLICT (account_id, installation_id) DO UPDATE
    SET first_seen_at = least(cloud_installations_seen.first_seen_at, excluded.first_seen_at),
        last_seen_at = greatest(cloud_installations_seen.last_seen_at, excluded.last_seen_at)
    RETURNING account_id, first_seen_at
)
UPDATE accounts a SET activated_at = sqlc.arg(seen_at)
FROM observed o
WHERE a.id = o.account_id AND a.activated_at IS NULL
  AND (sqlc.arg(seen_at) AT TIME ZONE 'UTC')::date > (o.first_seen_at AT TIME ZONE 'UTC')::date
  AND sqlc.arg(seen_at) BETWEEN a.created_at AND a.created_at + interval '7 days';

-- name: ActivationFunnel :many
SELECT first_sign_in_provider AS provider, count(*) AS accounts,
       count(*) FILTER (WHERE activated_at IS NOT NULL) AS activated
FROM accounts WHERE created_at BETWEEN sqlc.arg(since)::timestamptz AND sqlc.arg(as_of)::timestamptz
GROUP BY first_sign_in_provider ORDER BY first_sign_in_provider;

-- name: CloudMarketingOptIns :one
SELECT count(*) FROM accounts WHERE marketing_opt_in;

-- name: CloudProAccounts :one
SELECT count(DISTINCT account_id) FROM subscriptions
WHERE (source = 'complimentary' AND ends_at > sqlc.arg(as_of)::timestamptz)
   OR (source = 'stripe' AND status IN ('active', 'trialing', 'past_due')
       AND price_id IN (sqlc.arg(monthly)::text, sqlc.arg(annual)::text));

-- name: StartProLapse :exec
UPDATE accounts SET pro_ended_at = sqlc.arg(as_of)::timestamptz
WHERE id = sqlc.arg(account_id) AND state = 'active' AND pro_ended_at IS NULL;

-- name: EndProLapse :exec
UPDATE accounts SET pro_ended_at = NULL WHERE id = $1 AND pro_ended_at IS NOT NULL;

-- name: DueOperationalMail :many
WITH pro_lapses AS (
    SELECT a.id, a.pro_ended_at, coalesce((
        SELECT min(t.delete_after) FROM computers c JOIN tunnels t ON t.computer_id = c.id
        WHERE c.account_id = a.id AND t.state = 'active'
    ), a.pro_ended_at + interval '7 days') AS delete_at
    FROM accounts a WHERE a.state = 'active' AND a.pro_ended_at IS NOT NULL
), candidates AS (
    SELECT id AS account_id, 'pro_ended'::text AS kind, ''::text AS computer_id, pro_ended_at AS since, delete_at
    FROM pro_lapses WHERE sqlc.arg(as_of)::timestamptz < delete_at
    UNION ALL
    SELECT id, 'pro_reminder', '', pro_ended_at, delete_at FROM pro_lapses
    WHERE sqlc.arg(as_of)::timestamptz >= delete_at - interval '2 days' AND sqlc.arg(as_of) < delete_at
    UNION ALL
    SELECT c.account_id, 'offline', c.id, greatest(t.created_at, c.last_control_at),
           greatest(t.created_at, c.last_control_at) + interval '14 days'
    FROM tunnels t JOIN computers c ON c.id = t.computer_id JOIN accounts a ON a.id = c.account_id
    WHERE a.state = 'active' AND t.state = 'active' AND t.delete_after IS NULL
      AND c.unlinked_at IS NULL AND c.remote_enabled
      AND greatest(t.created_at, c.last_control_at) <= sqlc.arg(as_of)::timestamptz - interval '10 days'
      AND greatest(t.created_at, c.last_control_at) > sqlc.arg(as_of)::timestamptz - interval '14 days'
), identified AS (
    SELECT *, concat(kind, '/', computer_id, '/', extract(epoch FROM since)::text) AS delivery_key
    FROM candidates
)
SELECT i.account_id, i.kind, i.computer_id, i.since, i.delete_at::timestamptz AS delete_at, i.delivery_key::text AS delivery_key, a.email,
       coalesce(c.name, '')::text AS computer_name
FROM identified i JOIN accounts a ON a.id = i.account_id
LEFT JOIN computers c ON c.id = i.computer_id
WHERE (sqlc.arg(for_account)::text = '' OR i.account_id = sqlc.arg(for_account))
  AND NOT EXISTS (
    SELECT 1 FROM cloud_email_deliveries d WHERE d.account_id = i.account_id AND d.delivery_key = i.delivery_key
)
ORDER BY i.account_id, i.kind, i.computer_id;

-- name: LockOperationalMailAccount :one
SELECT id FROM accounts WHERE id = $1 FOR UPDATE;

-- name: CompleteOperationalMail :execrows
INSERT INTO cloud_email_deliveries (account_id, delivery_key) VALUES ($1, $2) ON CONFLICT DO NOTHING;
