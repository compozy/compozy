-- SPDX-License-Identifier: FSL-1.1-MIT

-- +goose Up
CREATE TABLE auth_ip_rate_windows (
    scope text NOT NULL,
    client_ip_sha256 bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    requests integer NOT NULL,
    PRIMARY KEY (scope, client_ip_sha256, expires_at)
);
CREATE INDEX auth_ip_rate_windows_expiry_idx ON auth_ip_rate_windows (expires_at);

-- +goose Down
DROP TABLE auth_ip_rate_windows;
