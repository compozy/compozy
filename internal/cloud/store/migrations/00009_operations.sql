-- SPDX-License-Identifier: FSL-1.1-MIT
-- +goose Up
ALTER TABLE accounts ADD COLUMN activated_at timestamptz;
ALTER TABLE accounts ADD COLUMN pro_ended_at timestamptz;
ALTER TABLE deleted_accounts ADD COLUMN security_archive_id bigint;
ALTER TABLE deleted_accounts ALTER COLUMN security_archive_id SET DEFAULT nextval('security_events_id_seq');
UPDATE deleted_accounts SET security_archive_id = nextval('security_events_id_seq')
WHERE security_archive_id IS NULL;

CREATE TABLE cloud_installations_seen (
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    installation_id text NOT NULL,
    first_seen_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    PRIMARY KEY (account_id, installation_id)
);

CREATE TABLE cloud_email_deliveries (
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    delivery_key text NOT NULL,
    PRIMARY KEY (account_id, delivery_key)
);

CREATE TABLE cloud_retired_signing_keys (
    kid text PRIMARY KEY,
    retired_at timestamptz NOT NULL
);

CREATE TABLE cloud_restores (
    restore_point timestamptz PRIMARY KEY,
    archive_through timestamptz NOT NULL,
    applied_at timestamptz,
    completed_at timestamptz
);
CREATE UNIQUE INDEX cloud_restores_one_pending ON cloud_restores ((true)) WHERE completed_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION cloud_drop_restored_token_instances(restore_point timestamptz) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    IF to_regclass('account_token_instances') IS NOT NULL THEN
        DELETE FROM account_token_instances WHERE last_seen_at > restore_point;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION cloud_revoke_restored_account_tokens(owner_id text, target_id text, revoked timestamptz)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    IF to_regclass('account_tokens') IS NOT NULL THEN
        UPDATE account_tokens SET revoked_at = revoked
        WHERE account_id = owner_id AND revoked_at IS NULL AND (target_id IS NULL OR id = target_id);
    ELSIF target_id IS NOT NULL THEN
        RAISE EXCEPTION 'account-token revocation requires the account-token schema';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION cloud_revoke_restored_account_tokens(text, text, timestamptz);
DROP FUNCTION cloud_drop_restored_token_instances(timestamptz);
DROP TABLE cloud_restores, cloud_retired_signing_keys, cloud_email_deliveries, cloud_installations_seen;
ALTER TABLE accounts DROP COLUMN pro_ended_at, DROP COLUMN activated_at;
ALTER TABLE deleted_accounts DROP COLUMN security_archive_id;
