-- +goose Up
ALTER TABLE subscriptions
    ADD COLUMN last_invoice_id text,
    ADD COLUMN refundable_payment boolean NOT NULL DEFAULT false;
ALTER TABLE accounts ADD COLUMN billing_reconciled_at timestamptz;

CREATE TABLE stripe_events (
    event_id text PRIMARY KEY,
    type text NOT NULL,
    received_at timestamptz NOT NULL,
    processed_at timestamptz NOT NULL
);

CREATE TABLE billing_checkouts (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    interval text NOT NULL CHECK (interval IN ('month', 'year')),
    hostname text
);
CREATE INDEX billing_checkouts_account_idx ON billing_checkouts (account_id);

-- +goose Down
DROP TABLE billing_checkouts, stripe_events;
ALTER TABLE subscriptions DROP COLUMN last_invoice_id, DROP COLUMN refundable_payment;
ALTER TABLE accounts DROP COLUMN billing_reconciled_at;
