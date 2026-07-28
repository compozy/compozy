package task

import "time"

// RunIdempotency is the durable deduplication record for non-human run ingress.
type RunIdempotency struct {
	IdempotencyKey string    `json:"idempotency_key"`
	RunID          string    `json:"run_id"`
	Origin         Origin    `json:"origin"`
	CreatedAt      time.Time `json:"created_at"`
}
