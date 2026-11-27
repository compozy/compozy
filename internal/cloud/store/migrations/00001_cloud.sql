-- +goose Up
CREATE TABLE accounts (
    id text PRIMARY KEY,
    email text NOT NULL UNIQUE CHECK (email <> '' AND email = lower(btrim(email))),
    name text NOT NULL,
    state text NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'suspended', 'deleting')),
    suspended_at timestamptz,
    first_sign_in_provider text NOT NULL CHECK (first_sign_in_provider IN ('google', 'github')),
    marketing_opt_in boolean NOT NULL DEFAULT false,
    marketing_consent_at timestamptz,
    marketing_consent_source text CHECK (marketing_consent_source IN ('sign_in', 'cli', 'settings', 'unsubscribe')),
    contact_synced_at timestamptz,
    cloud_routes_max integer NOT NULL DEFAULT 10 CHECK (cloud_routes_max >= 0),
    security_epoch bigint NOT NULL DEFAULT 1 CHECK (security_epoch > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz
);

CREATE TABLE identities (
    provider text NOT NULL CHECK (provider IN ('google', 'github')),
    subject text NOT NULL,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    email text NOT NULL CHECK (email <> '' AND email = lower(btrim(email))),
    created_at timestamptz NOT NULL DEFAULT now(),
    last_sign_in_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);
CREATE INDEX identities_account_idx ON identities (account_id);

CREATE TABLE devices (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('desktop', 'cli', 'browser')),
    installation_id text,
    browser_binding_sha256 bytea UNIQUE,
    admin boolean NOT NULL DEFAULT false,
    revoked_at timestamptz,
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, account_id)
);
CREATE INDEX devices_account_idx ON devices (account_id);

CREATE TABLE device_keys (
    jkt text PRIMARY KEY,
    device_id text NOT NULL REFERENCES devices ON DELETE CASCADE,
    public_jwk jsonb NOT NULL,
    origin text,
    state text NOT NULL CHECK (state IN ('pending', 'active', 'revoked')),
    pending_expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    activated_at timestamptz,
    revoked_at timestamptz,
    UNIQUE (device_id, jkt)
);
CREATE UNIQUE INDEX device_keys_live_origin_idx
    ON device_keys (device_id, origin, state) NULLS NOT DISTINCT WHERE state <> 'revoked';
CREATE INDEX device_keys_pending_idx ON device_keys (pending_expires_at) WHERE state = 'pending';

CREATE TABLE device_grants (
    device_code_sha256 bytea PRIMARY KEY,
    user_code text NOT NULL UNIQUE,
    provider text CHECK (provider IN ('google', 'github')),
    client_name text NOT NULL,
    client_platform text NOT NULL,
    installation_id text NOT NULL,
    request_location text,
    state text NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending', 'authorized', 'redeemed', 'cancelled', 'expired')),
    oauth_state_sha256 bytea UNIQUE,
    pkce_verifier_enc bytea,
    activation_nonce_sha256 bytea,
    account_id text REFERENCES accounts ON DELETE CASCADE,
    failed_entries integer NOT NULL DEFAULT 0 CHECK (failed_entries >= 0),
    expires_at timestamptz NOT NULL,
    last_poll_at timestamptz
);
CREATE INDEX device_grants_expiry_idx ON device_grants (expires_at);

CREATE TABLE enrollment_tokens (
    token_sha256 bytea PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    device_code_sha256 bytea NOT NULL REFERENCES device_grants ON DELETE CASCADE,
    provider text NOT NULL CHECK (provider IN ('google', 'github')),
    max_age_zero boolean NOT NULL DEFAULT false,
    expires_at timestamptz NOT NULL,
    used_at timestamptz
);
CREATE INDEX enrollment_tokens_expiry_idx ON enrollment_tokens (expires_at);

CREATE TABLE device_sessions (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    device_id text NOT NULL,
    jkt text NOT NULL,
    refresh_token_sha256 bytea NOT NULL UNIQUE,
    provider text NOT NULL CHECK (provider IN ('google', 'github')),
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now(),
    rotated_at timestamptz,
    revoked_at timestamptz,
    FOREIGN KEY (device_id, account_id) REFERENCES devices (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (device_id, jkt) REFERENCES device_keys (device_id, jkt) ON DELETE CASCADE
);
CREATE UNIQUE INDEX device_sessions_active_idx ON device_sessions (device_id) WHERE revoked_at IS NULL;
CREATE INDEX device_sessions_account_idx ON device_sessions (account_id);

CREATE TABLE cloud_web_sessions (
    id_sha256 bytea PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    browser_device_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    FOREIGN KEY (browser_device_id, account_id) REFERENCES devices (id, account_id) ON DELETE CASCADE
);
CREATE INDEX cloud_web_sessions_account_idx ON cloud_web_sessions (account_id);
CREATE INDEX cloud_web_sessions_expiry_idx ON cloud_web_sessions (expires_at);

CREATE TABLE computers (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    name text NOT NULL,
    platform text NOT NULL,
    version text NOT NULL,
    computer_jwk jsonb NOT NULL,
    tls_root_sha256 text NOT NULL,
    remote_enabled boolean NOT NULL DEFAULT false,
    plan_limited boolean NOT NULL DEFAULT false,
    last_session_at timestamptz,
    last_control_at timestamptz,
    linked_at timestamptz NOT NULL DEFAULT now(),
    unlinked_at timestamptz,
    UNIQUE (id, account_id)
);
CREATE INDEX computers_account_idx ON computers (account_id);
CREATE INDEX computers_enabled_idx ON computers (account_id)
    WHERE remote_enabled AND NOT plan_limited AND unlinked_at IS NULL;

CREATE TABLE device_scopes (
    device_id text NOT NULL REFERENCES devices ON DELETE CASCADE,
    computer_id text REFERENCES computers ON DELETE CASCADE,
    scopes text[] NOT NULL,
    UNIQUE NULLS NOT DISTINCT (device_id, computer_id)
);

CREATE TABLE computer_approved_keys (
    computer_id text NOT NULL REFERENCES computers ON DELETE CASCADE,
    jkt text NOT NULL REFERENCES device_keys ON DELETE CASCADE,
    reported_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (computer_id, jkt)
);

CREATE TABLE slot_reservations (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    device_id text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    released_at timestamptz,
    UNIQUE (id, account_id, device_id),
    FOREIGN KEY (device_id, account_id) REFERENCES devices (id, account_id) ON DELETE CASCADE
);
CREATE INDEX slot_reservations_active_idx ON slot_reservations (account_id, expires_at)
    WHERE consumed_at IS NULL AND released_at IS NULL;

CREATE TABLE computer_endpoints (
    computer_id text NOT NULL REFERENCES computers ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind = 'direct'),
    address text NOT NULL,
    reported_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (computer_id, kind, address)
);

CREATE TABLE link_tokens (
    token_sha256 bytea PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    device_id text NOT NULL,
    reservation_id text,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    used_by_jkt text,
    result_computer_id text,
    FOREIGN KEY (device_id, account_id) REFERENCES devices (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (reservation_id, account_id, device_id)
        REFERENCES slot_reservations (id, account_id, device_id) ON DELETE SET NULL (reservation_id),
    FOREIGN KEY (result_computer_id, account_id) REFERENCES computers (id, account_id) ON DELETE CASCADE
);
CREATE INDEX link_tokens_expiry_idx ON link_tokens (expires_at);

CREATE TABLE device_approvals (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    computer_id text NOT NULL,
    device_id text NOT NULL,
    jkt text NOT NULL,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'approved', 'denied', 'expired')),
    created_at timestamptz NOT NULL DEFAULT now(),
    decided_at timestamptz,
    decided_by_device text,
    decision_jws text,
    applied_result text,
    FOREIGN KEY (computer_id, account_id) REFERENCES computers (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (device_id, account_id) REFERENCES devices (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (device_id, jkt) REFERENCES device_keys (device_id, jkt) ON DELETE CASCADE,
    FOREIGN KEY (decided_by_device, account_id) REFERENCES devices (id, account_id)
        ON DELETE SET NULL (decided_by_device)
);
CREATE INDEX device_approvals_account_idx ON device_approvals (account_id, state, decided_at);
CREATE UNIQUE INDEX device_approvals_pending_idx
    ON device_approvals (computer_id, device_id, jkt) WHERE state = 'pending';

CREATE TABLE grants_issued (
    jti text PRIMARY KEY,
    computer_id text NOT NULL REFERENCES computers ON DELETE CASCADE,
    device_id text NOT NULL REFERENCES devices ON DELETE CASCADE,
    route text NOT NULL CHECK (route IN ('direct', 'ssh', 'cloud')),
    issued_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX grants_issued_at_idx ON grants_issued (issued_at);
CREATE INDEX grants_issued_device_idx ON grants_issued (device_id, issued_at);

CREATE TABLE subscriptions (
    id text PRIMARY KEY,
    source text NOT NULL CHECK (source IN ('stripe', 'complimentary')),
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    stripe_customer_id text,
    status text NOT NULL,
    price_id text,
    interval text CHECK (interval IN ('month', 'year')),
    current_period_end timestamptz,
    cancel_at_period_end boolean NOT NULL DEFAULT false,
    last_payment_at timestamptz,
    ends_at timestamptz,
    granted_by text,
    stripe_synced_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subscriptions_account_idx ON subscriptions (account_id);

CREATE TABLE browser_codes (
    code_sha256 bytea PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    device_id text NOT NULL,
    computer_id text NOT NULL,
    jkt text NOT NULL,
    code_challenge text NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    FOREIGN KEY (computer_id, account_id) REFERENCES computers (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (device_id, account_id) REFERENCES devices (id, account_id) ON DELETE CASCADE,
    FOREIGN KEY (device_id, jkt) REFERENCES device_keys (device_id, jkt) ON DELETE CASCADE
);
CREATE INDEX browser_codes_expiry_idx ON browser_codes (expires_at);

CREATE TABLE tunnels (
    computer_id text PRIMARY KEY REFERENCES computers ON DELETE CASCADE,
    cf_tunnel_id text,
    hostname text NOT NULL UNIQUE,
    state text NOT NULL CHECK (state IN ('reserved', 'provisioning', 'active', 'deleting', 'deleted', 'failed')),
    generation bigint NOT NULL DEFAULT 1 CHECK (generation > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    delete_after timestamptz
);
CREATE INDEX tunnels_delete_after_idx ON tunnels (delete_after) WHERE delete_after IS NOT NULL;
