package globaldb

import (
	"context"
	"database/sql"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ExtensionRegistryRepo maps bundled migration queries without exposing generated types.
type ExtensionRegistryRepo struct{ queries *sqlcgen.Queries }

// NewExtensionRegistryRepo shares the registry's existing database handle.
func NewExtensionRegistryRepo(db *sql.DB) *ExtensionRegistryRepo {
	return &ExtensionRegistryRepo{queries: sqlcgen.New(db)}
}

// NewExtensionRegistryWriteRepo keeps migration queries in the caller-owned enrollment transaction.
func NewExtensionRegistryWriteRepo(tx *store.WriteTx) *ExtensionRegistryRepo {
	return &ExtensionRegistryRepo{queries: sqlcgen.New(tx)}
}

// Source returns the persisted ownership of a migration participant.
func (r *ExtensionRegistryRepo) Source(ctx context.Context, name string) (string, error) {
	return r.queries.GetBundledMigrationSource(ctx, name)
}

// HasIncompatibleState checks for state without a lossless successor translation.
func (r *ExtensionRegistryRepo) HasIncompatibleState(ctx context.Context, name string) (bool, error) {
	result, err := r.queries.HasIncompatibleBundledMigrationState(ctx, name)
	return result.Bool, err
}

// EnablementConflicts compares effective profile enablement before merging installations.
func (r *ExtensionRegistryRepo) EnablementConflicts(ctx context.Context, old, successor string) (bool, error) {
	result, err := r.queries.HasBundledMigrationEnablementConflict(ctx,
		sqlcgen.HasBundledMigrationEnablementConflictParams{Predecessor: old, Successor: successor})
	return result, err
}

// TransferState replaces only fresh defaults and preserves timestamps and independent successor scopes.
func (r *ExtensionRegistryRepo) TransferState(ctx context.Context, old, successor string, fresh bool) error {
	if fresh {
		if err := r.queries.DeleteBundledMigrationInstallations(ctx, successor); err != nil {
			return err
		}
		if err := r.queries.DeleteBundledMigrationEnablement(ctx, successor); err != nil {
			return err
		}
	}
	if err := r.queries.CopyBundledMigrationInstallations(ctx,
		sqlcgen.CopyBundledMigrationInstallationsParams{Successor: successor, Predecessor: old}); err != nil {
		return err
	}
	if err := r.queries.CopyBundledMigrationEnablement(ctx,
		sqlcgen.CopyBundledMigrationEnablementParams{Successor: successor, Predecessor: old}); err != nil {
		return err
	}
	return r.queries.DeleteSupersededBundledExtension(ctx, old)
}

// ExtensionBindingScope identifies one persisted credential cell, including inactive bindings.
type ExtensionBindingScope struct {
	ProfileID   string
	WorkspaceID string
}

// EnvBindingScopes returns all credential owners in deterministic order for orphan retirement.
func (r *ExtensionRegistryRepo) EnvBindingScopes(ctx context.Context, name string) ([]ExtensionBindingScope, error) {
	rows, err := r.queries.ListExtensionEnvBindingScopes(ctx, name)
	if err != nil {
		return nil, err
	}
	result := make([]ExtensionBindingScope, 0, len(rows))
	for _, row := range rows {
		result = append(result, ExtensionBindingScope{ProfileID: row.ProfileID, WorkspaceID: row.WorkspaceID})
	}
	return result, nil
}
