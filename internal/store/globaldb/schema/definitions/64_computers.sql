CREATE TABLE computers (
    id TEXT NOT NULL PRIMARY KEY,
    account_id TEXT NOT NULL,
    name TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    platform TEXT,
    version TEXT,
    tls_root_sha256 TEXT NOT NULL,
    weight INTEGER NOT NULL DEFAULT 50 CHECK (weight IN (0, 25, 50, 100)),
    added_at TEXT NOT NULL,
    last_connected_at TEXT
);

CREATE INDEX computers_account_id ON computers (account_id, id);

CREATE TABLE computer_routes (
    id TEXT NOT NULL PRIMARY KEY,
    computer_id TEXT NOT NULL REFERENCES computers (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('direct', 'ssh', 'cloud', 'relay')),
    address TEXT NOT NULL,
    position INTEGER NOT NULL,
    learned INTEGER NOT NULL DEFAULT 0,
    removed INTEGER NOT NULL DEFAULT 0,
    ssh_user TEXT,
    ssh_port INTEGER,
    ssh_alias TEXT,
    UNIQUE (computer_id, kind, address)
);
