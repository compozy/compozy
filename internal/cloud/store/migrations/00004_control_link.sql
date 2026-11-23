-- +goose Up
ALTER TABLE computers ADD COLUMN lease_sequence bigint NOT NULL DEFAULT 0, ADD COLUMN control_link_id text;

CREATE TABLE computer_proof_replay (
    jkt text NOT NULL,
    jti text NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (jkt, jti)
);
CREATE INDEX computer_proof_replay_expiry_idx ON computer_proof_replay (expires_at);

-- +goose Down
DROP TABLE computer_proof_replay;
ALTER TABLE computers DROP COLUMN lease_sequence, DROP COLUMN control_link_id;
