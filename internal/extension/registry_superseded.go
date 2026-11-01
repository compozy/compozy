package extensionpkg

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// Remove this N-1 bundled-name translation in v0.6.0 after the v0.5 migration window.
var supersededBundledExtensions = map[string]string{"dev-cycle": "spec-cycle"}

// SupersededBundledSuccessor identifies a released bundled package replacement.
func SupersededBundledSuccessor(name string) string { return supersededBundledExtensions[name] }

// MigrateSupersededBundled transfers compatible scoped state atomically and retains the old package files.
func (r *Registry) MigrateSupersededBundled(ctx context.Context, successor string, fresh bool) error {
	for old, replacement := range supersededBundledExtensions {
		if replacement != successor {
			continue
		}
		info, err := r.Get(old)
		if errors.Is(err, ErrExtensionNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Source != SourceBundled {
			continue
		}
		target, err := r.Get(successor)
		if err != nil {
			return err
		}
		if target.Source != SourceBundled {
			return fmt.Errorf("extension: successor %q is operator-owned", successor)
		}
		err = store.ExecuteWrite(ctx, r.db, func(ctx context.Context, tx *store.WriteTx) error {
			// Credential ownership and resource names cannot be translated without an explicit contract.
			var incompatible bool
			if err := tx.QueryRowContext(ctx, `SELECT
    EXISTS(SELECT 1 FROM extension_inputs WHERE extension = ?) OR
    EXISTS(SELECT 1 FROM extension_env_bindings WHERE extension_name = ?) OR
    EXISTS(SELECT 1 FROM extension_mcp_overrides WHERE extension = ?) OR
    EXISTS(SELECT 1 FROM extension_profile_markers WHERE extension_name = ?) OR
    EXISTS(SELECT 1 FROM extension_dev_links WHERE extension_name = ?) OR
    NOT EXISTS(SELECT 1 FROM extension_installations WHERE extension_name = ?)`, old, old, old, old, old, old).Scan(&incompatible); err != nil {
				return err
			}
			if incompatible {
				return errors.New("extension: retained scoped inputs, credentials or resources need operator migration")
			}
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

			if !fresh {
				var conflict bool
				if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM profiles p WHERE
     COALESCE((SELECT enabled FROM extension_profile_enablement WHERE extension_name = ? AND profile_id = p.id), 1)
     <> COALESCE((SELECT enabled FROM extension_profile_enablement WHERE extension_name = ? AND profile_id = p.id), 1))`,
					old, successor).Scan(&conflict); err != nil {
					return err
				}
				if conflict {
					return errors.New("extension: successor has conflicting profile enablement")
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
		})
		if err != nil {
			return fmt.Errorf(
				"extension %q superseded by %q; state retained; migrate state then run compozy extension remove %s --global: %w",
				old,
				successor,
				old,
				err,
			)
		}
		r.invalidateEnabledBundledNames()
	}
	return nil
}
