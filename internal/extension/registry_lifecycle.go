package extensionpkg

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (r *Registry) updateEnabled(name string, enabled bool) error {
	if err := r.checkReady("update extension enabled state"); err != nil {
		return err
	}

	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return err
	}
	lifecycleToken, err := store.NewID("extstate")
	if err != nil {
		return fmt.Errorf("extension: generate lifecycle token for %q: %w", trimmedName, err)
	}
	result, err := r.db.ExecContext(
		registryContext(),
		`UPDATE extensions SET enabled = ?, lifecycle_token = ? WHERE name = ?`,
		enabled,
		lifecycleToken,
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

// DisableIfLifecycleToken disables an extension only when the persisted lifecycle
// owner still matches the instance that requested cleanup.
func (r *Registry) DisableIfLifecycleToken(name string, expectedToken string) (bool, error) {
	if err := r.checkReady("conditionally disable extension"); err != nil {
		return false, err
	}
	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return false, err
	}
	trimmedToken := strings.TrimSpace(expectedToken)
	nextToken, err := store.NewID("extstate")
	if err != nil {
		return false, fmt.Errorf("extension: generate lifecycle token for %q: %w", trimmedName, err)
	}
	result, err := r.db.ExecContext(
		registryContext(),
		`UPDATE extensions SET enabled = 0, lifecycle_token = ? WHERE name = ? AND lifecycle_token = ?`,
		nextToken,
		trimmedName,
		trimmedToken,
	)
	if err != nil {
		return false, fmt.Errorf("extension: conditionally disable %q: %w", trimmedName, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("extension: inspect conditional disable for %q: %w", trimmedName, err)
	}
	if rows > 0 {
		r.invalidateEnabledBundledNames()
	}
	return rows > 0, nil
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
