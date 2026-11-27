CREATE TABLE remote_identity (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    computer_id TEXT NOT NULL,
    account_id TEXT NOT NULL,
    computer_key_ref TEXT NOT NULL,
    tls_root_ref TEXT NOT NULL,
    tls_root_sha256 TEXT NOT NULL,
    tls_leaf_not_after TEXT NOT NULL,
    linked_at TEXT NOT NULL,
    lease_sequence INTEGER NOT NULL DEFAULT 0,
    last_mint_at INTEGER NOT NULL DEFAULT 0,
    last_mint_device_id TEXT NOT NULL DEFAULT '',
    last_mint_jkt TEXT NOT NULL DEFAULT ''
);

CREATE TABLE remote_sessions (
    id TEXT PRIMARY KEY,
    credential_sha256 BLOB NOT NULL UNIQUE,
    device_id TEXT NOT NULL,
    device_jkt TEXT NOT NULL,
    scopes TEXT NOT NULL,
    route TEXT NOT NULL,
    browser INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    revoked_at INTEGER,
    signing_key_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_remote_sessions_device ON remote_sessions (device_id);
CREATE INDEX idx_remote_sessions_expiry ON remote_sessions (expires_at);

CREATE TABLE remote_device_connections (
    device_id TEXT NOT NULL PRIMARY KEY,
    last_seen_at INTEGER NOT NULL
);

CREATE TABLE remote_grant_replay (
    jti TEXT PRIMARY KEY,
    expires_at INTEGER NOT NULL
);

CREATE TABLE remote_device_approvals (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL,
    jkt TEXT NOT NULL,
    name TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('pending', 'approved', 'denied', 'expired')),
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    decided_at INTEGER,
    decision_jti TEXT NOT NULL DEFAULT '',
    UNIQUE (device_id, jkt)
);

CREATE TABLE remote_approval_replay (
    jti TEXT PRIMARY KEY,
    expires_at INTEGER NOT NULL
);

CREATE TABLE remote_security_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    applied_epoch INTEGER NOT NULL,
    devices_json TEXT NOT NULL,
    applied_at INTEGER NOT NULL
);
