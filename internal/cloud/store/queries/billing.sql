-- SPDX-License-Identifier: FSL-1.1-MIT

-- name: LockStripeSubscription :exec
SELECT pg_advisory_xact_lock(hashtext(sqlc.arg(subscription_id)::text));

-- name: InsertStripeEvent :execrows
INSERT INTO stripe_events (event_id, type, received_at, processed_at) VALUES ($1, $2, $3, $3)
ON CONFLICT DO NOTHING;

-- name: MarkStripeEventProcessed :exec
UPDATE stripe_events SET processed_at = $2 WHERE event_id = $1;

-- name: BillingSubscription :one
SELECT * FROM subscriptions WHERE id = $1;

-- name: SaveStripeSubscription :execrows
INSERT INTO subscriptions (
    id, account_id, source, stripe_customer_id, status, price_id, interval, current_period_end,
    cancel_at_period_end, last_payment_at, ends_at, last_invoice_id, refundable_payment, stripe_synced_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(account_id), 'stripe', sqlc.narg(stripe_customer_id), sqlc.arg(status), sqlc.narg(price_id),
    sqlc.narg(interval), sqlc.narg(current_period_end), sqlc.arg(cancel_at_period_end), sqlc.narg(last_payment_at),
    sqlc.narg(ends_at), sqlc.narg(last_invoice_id), sqlc.arg(refundable_payment), sqlc.arg(as_of), sqlc.arg(as_of)
)
ON CONFLICT (id) DO UPDATE
SET stripe_customer_id = excluded.stripe_customer_id, status = excluded.status, price_id = excluded.price_id,
    interval = excluded.interval, current_period_end = excluded.current_period_end,
    cancel_at_period_end = excluded.cancel_at_period_end, last_payment_at = excluded.last_payment_at,
    ends_at = excluded.ends_at, last_invoice_id = excluded.last_invoice_id, refundable_payment = excluded.refundable_payment,
    stripe_synced_at = excluded.stripe_synced_at, updated_at = excluded.updated_at
WHERE subscriptions.source = 'stripe' AND subscriptions.account_id = excluded.account_id;

-- name: BillingStripeSubscriptionIDs :many
SELECT id FROM subscriptions WHERE source = 'stripe' ORDER BY id;

-- name: ExpiredComplimentaryAccounts :many
SELECT DISTINCT s.account_id FROM subscriptions s
JOIN accounts a ON a.id = s.account_id
WHERE s.source = 'complimentary' AND s.ends_at <= sqlc.arg(as_of)::timestamptz AND a.state = 'active'
  AND (a.billing_reconciled_at IS NULL OR s.ends_at > a.billing_reconciled_at);

-- name: MarkComplimentaryReconciled :execrows
UPDATE accounts a SET billing_reconciled_at = sqlc.arg(as_of)
WHERE a.id = sqlc.arg(account_id) AND a.state = 'active'
  AND EXISTS (
    SELECT 1 FROM subscriptions s WHERE s.account_id = a.id AND s.source = 'complimentary'
      AND s.ends_at <= sqlc.arg(as_of)
      AND (a.billing_reconciled_at IS NULL OR s.ends_at > a.billing_reconciled_at)
  );

-- name: EndComplimentarySubscriptions :exec
UPDATE subscriptions SET ends_at = $2, updated_at = $2
WHERE account_id = $1 AND source = 'complimentary' AND ends_at > $2;

-- name: SetHostedPlanRetention :execrows
UPDATE tunnels t SET delete_after = sqlc.narg(delete_after)
FROM computers c
WHERE t.computer_id = c.id AND c.account_id = sqlc.arg(account_id) AND c.unlinked_at IS NULL
  AND t.state = 'active' AND (t.delete_after IS NULL) <> (sqlc.narg(delete_after)::timestamptz IS NULL);

-- name: ReleaseExcessFreeReservations :execrows
WITH ranked AS (
    SELECT id, row_number() OVER (ORDER BY expires_at, id) AS slot
    FROM slot_reservations
    WHERE account_id = sqlc.arg(account_id)::text AND expires_at > sqlc.arg(as_of)::timestamptz
      AND consumed_at IS NULL AND released_at IS NULL
), capacity AS (
    SELECT CASE WHEN EXISTS (
        SELECT 1 FROM computers
        WHERE account_id = sqlc.arg(account_id)::text AND remote_enabled AND NOT plan_limited AND unlinked_at IS NULL
    ) THEN 0 ELSE 1 END AS slots
)
UPDATE slot_reservations s SET released_at = sqlc.arg(as_of)
FROM ranked, capacity WHERE s.id = ranked.id AND ranked.slot > capacity.slots;

-- name: InsertBillingCheckout :exec
INSERT INTO billing_checkouts (id, account_id, interval, hostname) VALUES ($1, $2, $3, $4);

-- name: AccountBillingCheckout :one
SELECT * FROM billing_checkouts WHERE id = $1 AND account_id = $2 AND expired_at IS NULL;

-- name: BillingCheckouts :many
SELECT * FROM billing_checkouts WHERE account_id = $1 ORDER BY id;

-- name: BillingCheckoutInterval :one
SELECT interval FROM billing_checkouts WHERE id = $1;
