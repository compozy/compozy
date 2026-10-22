package clientstate

import (
	"context"
	"errors"
	"os"

	bolt "go.etcd.io/bbolt"
)

// ReadMigrationEntries reads existing state without initialization, mutation, or workspace resolution.
func ReadMigrationEntries(
	ctx context.Context,
	path string,
	ws WorkspaceID,
	domain string,
) (entries []Entry, resultErr error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{ReadOnly: true, Timeout: defaultOpenTimeout})
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(workspaceBucketName(ws))
		if bucket == nil {
			return nil
		}
		return appendDomainEntries(bucket.Bucket([]byte(domain)), domain, &entries)
	})
	return entries, err
}
