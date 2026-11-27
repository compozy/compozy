-- +goose Up
ALTER TABLE device_grants
    ADD COLUMN new_account boolean NOT NULL DEFAULT false,
    ADD COLUMN max_age_zero boolean NOT NULL DEFAULT false,
    ADD COLUMN failure_code text,
    ADD COLUMN poll_interval_seconds integer NOT NULL DEFAULT 5 CHECK (poll_interval_seconds >= 5);

-- Each browser gets its own single-use OAuth attempt for a pending grant.
CREATE TABLE cloud_oauth_attempts (
    state_sha256 bytea PRIMARY KEY,
    device_code_sha256 bytea REFERENCES device_grants ON DELETE CASCADE,
    browser_request jsonb,
    provider text NOT NULL CHECK (provider IN ('google', 'github')),
    cookie_sha256 bytea NOT NULL,
    pkce_verifier_enc bytea NOT NULL,
    nonce_sha256 bytea NOT NULL,
    max_age_zero boolean NOT NULL,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    CHECK ((device_code_sha256 IS NULL) <> (browser_request IS NULL))
);
CREATE INDEX cloud_oauth_attempts_expiry_idx ON cloud_oauth_attempts (expires_at);
CREATE INDEX cloud_oauth_attempts_grant_idx ON cloud_oauth_attempts (device_code_sha256);

CREATE TABLE cloud_dpop_replays (
    jkt text NOT NULL,
    jti text NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (jkt, jti)
);
CREATE INDEX cloud_dpop_replays_expiry_idx ON cloud_dpop_replays (expires_at);

CREATE TABLE deleted_accounts (
    account_id text PRIMARY KEY,
    deleted_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE cloud_dpop_replays, cloud_oauth_attempts, deleted_accounts;
ALTER TABLE device_grants DROP COLUMN poll_interval_seconds, DROP COLUMN failure_code,
    DROP COLUMN max_age_zero, DROP COLUMN new_account;
