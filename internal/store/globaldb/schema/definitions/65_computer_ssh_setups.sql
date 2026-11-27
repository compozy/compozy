CREATE TABLE computer_ssh_setups (
    id TEXT NOT NULL PRIMARY KEY,
    account_id TEXT NOT NULL,
    host_key TEXT NOT NULL,
    target_alias TEXT NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('add', 'update')),
    step TEXT NOT NULL CHECK (step IN ('admit', 'verify', 'connect', 'install', 'service', 'link', 'forward', 'done', 'cancelled')),
    release_version TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT '',
    binary_path TEXT NOT NULL DEFAULT '',
    service_warning TEXT NOT NULL DEFAULT '',
    reservation_id TEXT NOT NULL DEFAULT '',
    link_token_ref TEXT NOT NULL DEFAULT '',
    link_token_expires_at TEXT NOT NULL DEFAULT '',
    computer_id TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_detail TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX computer_ssh_setups_pending_host
    ON computer_ssh_setups (account_id, host_key)
    WHERE step NOT IN ('done', 'cancelled');
