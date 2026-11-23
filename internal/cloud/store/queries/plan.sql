-- name: LockPlanAccount :one
SELECT id FROM accounts WHERE id = $1 FOR UPDATE;

-- name: PlanUsage :one
SELECT
    (SELECT count(*) FROM computers
     WHERE account_id = sqlc.arg(account_id)::text AND remote_enabled AND NOT plan_limited
       AND unlinked_at IS NULL) AS enabled_computers,
    (SELECT count(*) FROM slot_reservations
     WHERE account_id = sqlc.arg(account_id)::text AND expires_at > sqlc.arg(as_of)::timestamptz
       AND consumed_at IS NULL AND released_at IS NULL) AS active_reservations;

-- name: PlanSubscriptions :many
SELECT source, price_id, status, ends_at FROM subscriptions WHERE account_id = $1;

-- name: AccountHostedRouteUsage :one
SELECT count(*) FROM tunnels t JOIN computers c ON c.id = t.computer_id
WHERE c.account_id = $1 AND c.unlinked_at IS NULL AND t.state IN ('reserved', 'provisioning', 'active');
