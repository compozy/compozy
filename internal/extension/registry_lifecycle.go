package extensionpkg

import (
	"errors"
	"fmt"
)

func (r *Registry) updateEnabled(name string, enabled bool) error {
	if err := r.checkReady("update extension enabled state"); err != nil {
		return err
	}

	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(
		registryContext(),
		`UPDATE extensions SET enabled = ? WHERE name = ?`,
		enabled,
		trimmedName,
	)
	if err != nil {
		return fmt.Errorf("extension: update enabled state for %q: %w", trimmedName, err)
	}

	if err := rowsAffectedNotFound(result, trimmedName); err != nil {
		return err
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
