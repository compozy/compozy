-- +goose Up
ALTER TABLE accounts ADD COLUMN contact_retry_at timestamptz;
ALTER TABLE billing_checkouts ADD COLUMN expired_at timestamptz;
ALTER TABLE cloud_web_sessions ADD COLUMN fresh_notice_event_id bigint;
ALTER TABLE computers ADD COLUMN installation_id text;
ALTER TABLE browser_codes ADD COLUMN request_location text;
ALTER TABLE deleted_accounts ADD COLUMN security_event_counts jsonb NOT NULL DEFAULT '{}';

CREATE TABLE account_deletion_jobs (
    account_id text PRIMARY KEY REFERENCES accounts ON DELETE CASCADE,
    requested_at timestamptz NOT NULL,
    step smallint NOT NULL DEFAULT 0,
    retry_at timestamptz NOT NULL
);
CREATE INDEX account_deletion_jobs_retry_idx ON account_deletion_jobs (retry_at, account_id);

CREATE TABLE account_deletion_identity_holds (
    subject_sha256 bytea PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE
);

CREATE TABLE account_contact_addresses (
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    email text NOT NULL,
    PRIMARY KEY (account_id, email)
);

CREATE TABLE account_email_outbox (
    event_id bigint PRIMARY KEY REFERENCES security_events ON DELETE CASCADE,
    retry_at timestamptz NOT NULL
);
CREATE INDEX account_email_outbox_retry_idx ON account_email_outbox (retry_at, event_id);

CREATE INDEX security_events_enrolled_idx ON security_events (account_id, at DESC, id DESC)
    WHERE type = 'device_enrolled';

-- The token tables are owned by the account-token migration.
-- +goose StatementBegin
CREATE FUNCTION revoke_lifecycle_tokens(owner_id text, revoked_time timestamptz) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
    IF to_regclass('account_tokens') IS NOT NULL THEN
        UPDATE account_tokens SET revoked_at = revoked_time WHERE account_id = owner_id AND revoked_at IS NULL;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION lifecycle_token_installations(since_time timestamptz) RETURNS bigint
LANGUAGE plpgsql STABLE AS $$
DECLARE
    total bigint;
BEGIN
    IF to_regclass('account_token_instances') IS NULL THEN
        RETURN 0;
    END IF;
    SELECT count(DISTINCT installation_id) INTO total
        FROM account_token_instances WHERE last_seen_at >= since_time;
    RETURN total;
END;
$$;
-- +goose StatementEnd

CREATE OR REPLACE VIEW security_event_retention AS
    SELECT NULL::timestamptz AS cutoff, NULL::text AS deleted_account_id WHERE false;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION compact_security_events() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    previous_retention text := current_setting('compozy.security_event_retention', true);
BEGIN
    -- A non-superuser can set this custom parameter at runtime.
    PERFORM set_config('compozy.security_event_retention', 'on', true);
    IF NEW.deleted_account_id IS NULL THEN
        WITH retired AS (
            DELETE FROM security_events WHERE at < NEW.cutoff RETURNING type
        )
        INSERT INTO security_event_counts (type, count)
            SELECT type, count(*) FROM retired GROUP BY type ORDER BY type
            ON CONFLICT (type) DO UPDATE SET count = security_event_counts.count + excluded.count;
    ELSE
        IF NOT EXISTS (SELECT 1 FROM accounts WHERE id = NEW.deleted_account_id AND state = 'deleting')
           OR NOT EXISTS (SELECT 1 FROM deleted_accounts WHERE account_id = NEW.deleted_account_id) THEN
            RAISE EXCEPTION 'security event compaction requires an account deletion' USING ERRCODE = '55000';
        END IF;
        WITH retired AS (
            DELETE FROM security_events WHERE account_id = NEW.deleted_account_id RETURNING type
        ), counts AS (
            SELECT type, count(*) AS total FROM retired GROUP BY type
        )
        UPDATE deleted_accounts
        SET security_event_counts = coalesce((SELECT jsonb_object_agg(type, total) FROM counts), '{}')
        WHERE account_id = NEW.deleted_account_id;
    END IF;
    PERFORM set_config('compozy.security_event_retention', coalesce(previous_retention, ''), true);
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
