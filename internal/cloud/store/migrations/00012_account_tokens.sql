-- SPDX-License-Identifier: FSL-1.1-MIT

-- +goose Up
CREATE TABLE account_tokens (
    id text PRIMARY KEY,
    account_id text NOT NULL REFERENCES accounts ON DELETE CASCADE,
    name text NOT NULL,
    token_sha256 bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL,
    last_used_at timestamptz,
    revoked_at timestamptz
);
CREATE INDEX account_tokens_account_idx ON account_tokens (account_id);

CREATE TABLE account_token_instances (
    token_id text NOT NULL REFERENCES account_tokens ON DELETE CASCADE,
    installation_id text NOT NULL,
    last_seen_at timestamptz NOT NULL,
    PRIMARY KEY (token_id, installation_id)
);

-- +goose Down
DROP TABLE account_token_instances;
DROP TABLE account_tokens;
