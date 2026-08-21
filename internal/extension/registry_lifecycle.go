package extensionpkg

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

func (r *Registry) updateEnabled(name string, enabled bool) (resultErr error) {
	if err := r.checkReady("update extension enabled state"); err != nil {
		return err
	}

	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(registryContext(), nil)
	if err != nil {
		return fmt.Errorf("extension: begin enabled-state update for %q: %w", trimmedName, err)
	}
	defer func() {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, fmt.Errorf("extension: roll back enabled-state update: %w", rollbackErr))
		}
	}()

	var exists bool
	if err := tx.QueryRowContext(
		registryContext(),
		`SELECT EXISTS(SELECT 1 FROM extensions WHERE name = ?)`,
		trimmedName,
	).Scan(&exists); err != nil {
		return fmt.Errorf("extension: check enabled-state target %q: %w", trimmedName, err)
	}
	if !exists {
		return &ExtensionNotFoundError{Name: trimmedName}
	}

	if enabled {
		_, err = tx.ExecContext(
			registryContext(),
			`DELETE FROM extension_profile_enablement WHERE extension_name = ? AND profile_id = ?`,
			trimmedName,
			store.DefaultProfileID,
		)
	} else {
		_, err = tx.ExecContext(
			registryContext(),
			`INSERT INTO extension_profile_enablement (extension_name, profile_id, enabled)
			 VALUES (?, ?, 0)
			 ON CONFLICT(extension_name, profile_id) DO UPDATE SET enabled = 0`,
			trimmedName,
			store.DefaultProfileID,
		)
	}
	if err != nil {
		return fmt.Errorf("extension: update enabled state for %q: %w", trimmedName, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("extension: commit enabled-state update for %q: %w", trimmedName, err)
	}
	r.invalidateEnabledBundledNames()
	return nil
}

func (r *Registry) checkReady(action string) error {
	if r == nil {
		return errors.New("extension: registry is required")
	}
	if r.db == nil {
		return fmt.Errorf("extension: %s database is required", action)
	}
	return nil
}
