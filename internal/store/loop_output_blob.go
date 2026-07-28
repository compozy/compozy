package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// LoopOutputBlobExecutor is the SQL surface required to persist one loop output blob.
type LoopOutputBlobExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// UpsertLoopOutputBlob persists a content-addressed loop output and refreshes its usage timestamp.
func UpsertLoopOutputBlob(
	ctx context.Context,
	exec LoopOutputBlobExecutor,
	outputRef string,
	payload []byte,
	at time.Time,
) error {
	if exec == nil {
		return fmt.Errorf("store: upsert loop output blob: executor is required")
	}
	ref := strings.TrimSpace(outputRef)
	if ref == "" {
		return fmt.Errorf("store: upsert loop output blob: output_ref is required")
	}
	if len(payload) == 0 {
		return fmt.Errorf("store: upsert loop output blob %q: payload is required", ref)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	timestamp := FormatTimestamp(at)
	if _, err := exec.ExecContext(
		ctx,
		`INSERT INTO loop_output_blobs (output_ref, payload_json, byte_size, created_at, last_used_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(output_ref) DO UPDATE SET
		   last_used_at = excluded.last_used_at`,
		ref,
		string(payload),
		len(payload),
		timestamp,
		timestamp,
	); err != nil {
		return fmt.Errorf("store: upsert loop output blob %q: %w", ref, err)
	}
	return nil
}
