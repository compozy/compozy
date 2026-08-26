package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// GetCallPayloads loads a bounded set of call blobs in one statement.
func (g *CallRepo) GetCallPayloads(
	ctx context.Context,
	workspaceID string,
	refs []string,
) (payloads map[string][]byte, err error) {
	if err := g.checkReady(ctx, "get call payloads"); err != nil {
		return nil, err
	}
	unique := make([]string, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return nil, errors.New("store: call payload ref is required")
		}
		if _, exists := seen[ref]; exists {
			continue
		}
		seen[ref] = struct{}{}
		unique = append(unique, ref)
	}
	payloads = make(map[string][]byte, len(unique))
	if len(unique) == 0 {
		return payloads, nil
	}
	args := make([]any, 0, len(unique)+1)
	args = append(args, strings.TrimSpace(workspaceID))
	for _, ref := range unique {
		args = append(args, ref)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(unique)), ",")
	rows, err := g.db.QueryContext(ctx, `SELECT ref, bytes, byte_size FROM payload_blobs
		WHERE workspace_id = ? AND ref IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: get call payloads: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "call payloads") }()
	for rows.Next() {
		var ref string
		var payload []byte
		var byteSize int64
		if scanErr := rows.Scan(&ref, &payload, &byteSize); scanErr != nil {
			return nil, fmt.Errorf("store: scan call payload: %w", scanErr)
		}
		verified, verifyErr := verifyCallBlob("payload", ref, payload, &byteSize)
		if verifyErr != nil {
			return nil, verifyErr
		}
		payloads[ref] = verified
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate call payloads: %w", err)
	}
	for _, ref := range unique {
		if _, ok := payloads[ref]; !ok {
			return nil, fmt.Errorf("store: get call payload %q: %w", ref, sql.ErrNoRows)
		}
	}
	return payloads, nil
}
