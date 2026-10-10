CREATE TABLE session_prompt_reply_watches (
    id TEXT PRIMARY KEY NOT NULL,
    workspace_id TEXT NOT NULL,
    sender_session_id TEXT NOT NULL,
    target_workspace_id TEXT NOT NULL,
    target_session_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    admission_id TEXT NOT NULL,
    turn_id TEXT,
    queue_entry_id TEXT,
    delivered_input_id TEXT,
    abandon_reason TEXT CHECK (abandon_reason IN ('sender_gone', 'send_failed')),
    hop INTEGER NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('armed', 'fired', 'delivered', 'abandoned')),
    outcome TEXT CHECK (outcome IN ('completed', 'failed', 'canceled', 'dropped', 'unknown')),
    reply_text TEXT,
    reply_truncated INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    fired_at TEXT,
    delivered_at TEXT,
    UNIQUE (target_session_id, message_id)
);
CREATE INDEX idx_reply_watches_sender_state ON session_prompt_reply_watches(sender_session_id, state);
CREATE INDEX idx_reply_watches_target_turn ON session_prompt_reply_watches(target_session_id, turn_id);
CREATE INDEX idx_reply_watches_target_queue ON session_prompt_reply_watches(target_session_id, queue_entry_id);
