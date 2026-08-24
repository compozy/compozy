-- +goose Up
DROP INDEX IF EXISTS memory_decisions_idempotency_key;
CREATE UNIQUE INDEX memory_decisions_idempotency_key
	ON memory_decisions (profile_id, idempotency_key);

-- +goose Down
DROP INDEX IF EXISTS memory_decisions_idempotency_key;
CREATE UNIQUE INDEX memory_decisions_idempotency_key
	ON memory_decisions (idempotency_key);
