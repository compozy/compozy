package extensionpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
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
	var source string
	err := tx.QueryRowContext(ctx, `SELECT source FROM extensions WHERE name = ?`, old).Scan(&source)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if source != SourceBundled.String() {
		return nil
	}
	if err := tx.QueryRowContext(ctx, `SELECT source FROM extensions WHERE name = ?`, successor).
		Scan(&source); err != nil {
		return err
	}
	if source != SourceBundled.String() {
		return fmt.Errorf("%w: successor is operator-owned", errBundledMigrationRetained)
	}

	if err := validateBundledMigrationState(ctx, tx, old, successor, fresh); err != nil {
		return err
	}
	return transferBundledMigrationState(ctx, tx, old, successor, fresh)
}

// Unsafe translation retains predecessor state rather than silently replacing successor settings.
func validateBundledMigrationState(ctx context.Context, tx *store.WriteTx, old, successor string, fresh bool) error {
	// Credential ownership and resource names cannot be translated without an explicit contract.
	var incompatible bool
	if err := tx.QueryRowContext(ctx, `SELECT
    EXISTS(SELECT 1 FROM extension_inputs WHERE extension = ?) OR
    EXISTS(SELECT 1 FROM extension_env_bindings WHERE extension_name = ?) OR
    EXISTS(SELECT 1 FROM extension_mcp_overrides WHERE extension = ?) OR
    EXISTS(SELECT 1 FROM extension_profile_markers WHERE extension_name = ?) OR
    EXISTS(SELECT 1 FROM extension_dev_links WHERE extension_name = ?) OR
    NOT EXISTS(SELECT 1 FROM extension_installations WHERE extension_name = ?)`,
		old, old, old, old, old, old).Scan(&incompatible); err != nil {
		return err
	}
	if incompatible {
		return fmt.Errorf(
			"%w: scoped inputs, credentials or resources need operator migration",
			errBundledMigrationRetained,
		)
	}
	if !fresh {
		var conflict bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM profiles p WHERE
     COALESCE((SELECT enabled FROM extension_profile_enablement
 WHERE extension_name = ? AND profile_id = p.id), 1)
     <> COALESCE((SELECT enabled FROM extension_profile_enablement
 WHERE extension_name = ? AND profile_id = p.id), 1))`,
			old, successor).Scan(&conflict); err != nil {
			return err
		}
		if conflict {
			return fmt.Errorf("%w: successor has conflicting profile enablement", errBundledMigrationRetained)
		}
	}

	return nil
}

// Fresh defaults are replaced inside enrollment, so an interrupted boot cannot commit widened scopes.
func transferBundledMigrationState(ctx context.Context, tx *store.WriteTx, old, successor string, fresh bool) error {
	if fresh {
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM extension_installations WHERE extension_name = ?`,
			successor,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM extension_profile_enablement WHERE extension_name = ?`,
			successor,
		); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO extension_installations (extension_name, profile_id, workspace_id, created_at)
    SELECT ?, profile_id, workspace_id, created_at FROM extension_installations WHERE extension_name = ?
    ON CONFLICT(extension_name, profile_id, workspace_id) DO NOTHING`,
		successor,
		old,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO extension_profile_enablement (extension_name, profile_id, enabled)
    SELECT ?, profile_id, enabled FROM extension_profile_enablement WHERE extension_name = ?
    ON CONFLICT(extension_name, profile_id) DO NOTHING`,
		successor,
		old,
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM extensions WHERE name = ? AND source = 'bundled'`, old)
	return err
}
