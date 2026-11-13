package clientstate

import (
	"bytes"
	"context"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// DomainEntriesByPrefix reads only matching keys in one transaction for daemon-selected workspaces.
func (e *Engine) DomainEntriesByPrefix(
	ctx context.Context, workspaces []WorkspaceID, domain, prefix string,
) (map[WorkspaceID][]Entry, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if err := validateDomain(domain); err != nil {
		return nil, err
	}
	if err := validateKey(prefix); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, ErrClosed
	}
	entries := make(map[WorkspaceID][]Entry)
	err := e.store.db.View(func(tx *bolt.Tx) error {
		for _, ws := range workspaces {
			if err := contextError(ctx); err != nil {
				return err
			}
			workspace := tx.Bucket(workspaceBucketName(ws))
			if workspace == nil {
				continue
			}
			bucket := workspace.Bucket([]byte(domain))
			if bucket == nil {
				continue
			}
			cursor := bucket.Cursor()
			for key, value := cursor.Seek([]byte(prefix)); bytes.HasPrefix(key, []byte(prefix)); key, value = cursor.Next() {
				if err := contextError(ctx); err != nil {
					return err
				}
				entry, err := decodeEntry(domain, string(key), value)
				if err != nil {
					return err
				}
				if !entry.Deleted {
					entries[ws] = append(entries[ws], entry)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("clientstate: read domain %q prefix %q: %w", domain, prefix, err)
	}
	return entries, nil
}
