package extensionpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// Remove this N-1 bundled-name translation in v0.6.0 after the v0.5 migration window.
var supersededBundledExtensions = map[string]string{"dev-cycle": "spec-cycle"}

// SupersededBundledSuccessor identifies a released bundled package replacement.
func SupersededBundledSuccessor(name string) string { return supersededBundledExtensions[name] }

var errBundledMigrationRetained = errors.New("extension: superseded bundled state retained")

// ReconcileSupersededBundled retries retirement for an already enrolled successor.
func (r *Registry) ReconcileSupersededBundled(ctx context.Context, successor string) error {
	if err := r.checkReady("reconcile superseded bundled extensions"); err != nil {
		return err
	}
	err := store.ExecuteWrite(ctx, r.db, func(ctx context.Context, tx *store.WriteTx) error {
		return reconcileSupersededBundledTx(ctx, tx, successor, false)
	})
	if err == nil {
		r.invalidateEnabledBundledNames()
	}
	return err
}

// Only the current successor generation participates in the bundled-name translation.
func reconcileSupersededBundledTx(ctx context.Context, tx *store.WriteTx, successor string, fresh bool) error {
	for old, replacement := range supersededBundledExtensions {
		if replacement != successor {
			continue
		}
		err := migrateSupersededBundledTx(ctx, tx, old, successor, fresh)

		if err != nil {
			return fmt.Errorf("extension: migrate bundled %q to %q: %w", old, successor, err)
		}
	}
	return nil
}

// Source ownership and every refusal condition are checked before any predecessor state changes.
func migrateSupersededBundledTx(ctx context.Context, tx *store.WriteTx, old, successor string, fresh bool) error {
	repo := globaldb.NewExtensionRegistryWriteRepo(tx)
	source, err := repo.Source(ctx, old)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if source != SourceBundled.String() {
		return nil
	}
	source, err = repo.Source(ctx, successor)
	if err != nil {
		return err
	}
	if source != SourceBundled.String() {
		return fmt.Errorf("%w: successor is operator-owned", errBundledMigrationRetained)
	}

	if err := validateBundledMigrationState(ctx, tx, old, successor, fresh); err != nil {
		return err
	}
	return repo.TransferState(ctx, old, successor, fresh)
}

// Unsafe translation retains predecessor state rather than silently replacing successor settings.
func validateBundledMigrationState(ctx context.Context, tx *store.WriteTx, old, successor string, fresh bool) error {
	// Credential ownership and resource names cannot be translated without an explicit contract.
	repo := globaldb.NewExtensionRegistryWriteRepo(tx)
	incompatible, err := repo.HasIncompatibleState(ctx, old)
	if err != nil {
		return err
	}
	if incompatible {
		return fmt.Errorf(
			"%w: scoped inputs, credentials or resources need operator migration",
			errBundledMigrationRetained,
		)
	}
	if !fresh {
		conflict, err := repo.EnablementConflicts(ctx, old, successor)
		if err != nil {
			return err
		}
		if conflict {
			return fmt.Errorf("%w: successor has conflicting profile enablement", errBundledMigrationRetained)
		}
	}

	return nil
}
