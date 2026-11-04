package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/vault"
)

// VaultReader observes encrypted records without migrating, recovering or
// checkpointing the daemon's database. The vault shape is unchanged since the
// baseline, so validated older migration streams can be read before daemon boot.
type VaultReader struct{ db *sql.DB }

func OpenVaultReader(ctx context.Context, path string) (*VaultReader, error) {
	if ctx == nil {
		return nil, errors.New("store: vault reader context is required")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("store: vault database path is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &VaultReader{}, nil
		}
		return nil, fmt.Errorf("store: inspect vault database: %w", err)
	}
	db, err := store.OpenSQLiteReadPool(ctx, path)
	if err != nil {
		return nil, err
	}
	status, err := store.Status(ctx, db, MigrationStream())
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	if status.Version == 0 {
		return nil, errors.Join(store.ErrSchemaBehind, db.Close())
	}
	return &VaultReader{db: db}, nil
}

func (r *VaultReader) GetVaultSecret(ctx context.Context, ref string) (vault.Record, error) {
	if err := ctx.Err(); err != nil {
		return vault.Record{}, err
	}
	if r.db == nil {
		return vault.Record{}, vault.ErrSecretNotFound
	}
	return getVaultSecretWithExecutor(ctx, r.db, ref)
}

func (r *VaultReader) ListVaultSecrets(ctx context.Context, prefix string) ([]vault.Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.db == nil {
		return []vault.Record{}, nil
	}
	return listVaultSecretsWithExecutor(ctx, r.db, prefix)
}

func (r *VaultReader) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}
