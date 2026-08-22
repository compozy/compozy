package extensionpkg

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// IsEnabledForProfile returns the effective state for one installed or dev-linked extension.
// Enablement rows are exceptions: an absent row means enabled.
func (r *Registry) IsEnabledForProfile(name string, profileID string) (bool, error) {
	if err := r.checkReady("read extension profile enablement"); err != nil {
		return false, err
	}
	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return false, err
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return false, errors.New("extension: profile id is required")
	}
	var enabled bool
	err = r.db.QueryRowContext(registryContext(), `
		SELECT COALESCE((
			SELECT enabled
			FROM extension_profile_enablement
			WHERE extension_name = ? AND profile_id = ?
		), 1)
		WHERE EXISTS (SELECT 1 FROM extensions WHERE name = ?)
		   OR EXISTS (SELECT 1 FROM extension_dev_links WHERE extension_name = ?)`,
		trimmedName,
		profileID,
		trimmedName,
		trimmedName,
	).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, &ExtensionNotFoundError{Name: trimmedName}
	}
	if err != nil {
		return false, fmt.Errorf("extension: read profile enablement for %q: %w", trimmedName, err)
	}
	return enabled, nil
}

// SetEnabledForProfile persists only disabled exceptions; enabling removes the row.
func (r *Registry) SetEnabledForProfile(name string, profileID string, enabled bool) (resultErr error) {
	if err := r.checkReady("update extension enabled state"); err != nil {
		return err
	}

	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return err
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return errors.New("extension: profile id is required")
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
		`SELECT EXISTS(SELECT 1 FROM extensions WHERE name = ?)
		     OR EXISTS(SELECT 1 FROM extension_dev_links WHERE extension_name = ?)`,
		trimmedName,
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
			profileID,
		)
	} else {
		_, err = tx.ExecContext(
			registryContext(),
			`INSERT INTO extension_profile_enablement (extension_name, profile_id, enabled)
			 VALUES (?, ?, 0)
			 ON CONFLICT(extension_name, profile_id) DO UPDATE SET enabled = 0`,
			trimmedName,
			profileID,
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
