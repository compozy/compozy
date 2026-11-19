CREATE TABLE token_usage_receipts (
    profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL,
    turn_id TEXT NOT NULL CHECK (length(trim(turn_id)) > 0),
    instance_id TEXT NOT NULL DEFAULT '',
    day TEXT NOT NULL,
    total_tokens INTEGER NOT NULL DEFAULT 0 CHECK (total_tokens >= 0),
    usd_amount TEXT,
    cost_status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    ownership_unresolved INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (session_id, turn_id)
);

CREATE INDEX idx_token_usage_receipts_instance_day
    ON token_usage_receipts(profile_id, instance_id, day);

-- Cursor-backed daily admissions must not re-sort a whole day's receipts
-- under the global writer lock as the ledger grows.
CREATE INDEX idx_token_usage_receipts_budget_cursor
    ON token_usage_receipts(profile_id, instance_id, day, session_id, turn_id);

-- A replay cursor is advanced only after the matching receipt (if any) has
-- been accepted. It is intentionally not FK-bound to sessions so the fence
-- survives deletion and a later daemon boot can resume without rescanning
-- unbounded historical Done events.
CREATE TABLE daily_budget_recovery_cursors (
    session_id TEXT NOT NULL PRIMARY KEY,
    after_sequence INTEGER NOT NULL CHECK (after_sequence >= 0),
    updated_at TEXT NOT NULL
);
