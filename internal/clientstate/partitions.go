package clientstate

import (
	"context"
	"fmt"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// PartitionIDs enumerates retained partitions for daemon lifecycle reconciliation.
func (e *Engine) PartitionIDs(ctx context.Context) ([]WorkspaceID, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, ErrClosed
	}
	ids := []WorkspaceID{}
	err := e.store.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			if id, ok := strings.CutPrefix(string(name), "ws:"); ok {
				ids = append(ids, WorkspaceID(id))
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("clientstate: enumerate partitions: %w", err)
	}
	return ids, nil
}
