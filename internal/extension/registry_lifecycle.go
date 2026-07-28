package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"

	"slices"
	"strings"
)

func (r *Registry) updateEnabled(name string, enabled bool) error {
	if err := r.checkReady("update extension enabled state"); err != nil {
		return err
	}

	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return err
	}
	if !enabled {
		if err := r.ensureNoActiveBundles(trimmedName); err != nil {
			return err
		}
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

func (r *Registry) ensureNoActiveBundles(extensionName string) (err error) {
	recordsTableExists, err := sqliteTableExists(r.db, "resource_records")
	if err != nil {
		return fmt.Errorf("extension: inspect active bundle activation table for %q: %w", extensionName, err)
	}
	if !recordsTableExists {
		return nil
	}

	rows, err := r.db.QueryContext(
		registryContext(),
		`SELECT id, spec_json FROM resource_records WHERE kind = ?`,
		"bundle.activation",
	)
	if err != nil {
		return fmt.Errorf("extension: count active bundle activations for %q: %w", extensionName, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf("extension: close active bundle activation rows for %q: %w", extensionName, closeErr),
			)
		}
	}()

	activationIDs := []string{}
	trimmedName := strings.TrimSpace(extensionName)
	for rows.Next() {
		var id string
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return fmt.Errorf("extension: scan active bundle activation for %q: %w", extensionName, err)
		}
		var spec struct {
			ExtensionName string `json:"extension_name"`
		}
		if err := json.Unmarshal([]byte(raw), &spec); err != nil {
			return fmt.Errorf("extension: decode active bundle activation for %q: %w", extensionName, err)
		}
		if strings.TrimSpace(spec.ExtensionName) == trimmedName {
			activationIDs = append(activationIDs, strings.TrimSpace(id))
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("extension: iterate active bundle activations for %q: %w", extensionName, err)
	}
	if len(activationIDs) > 0 {
		slices.Sort(activationIDs)
		return fmt.Errorf(
			"%w: %q has active activation(s): %s",
			ErrExtensionHasActiveBundles,
			extensionName,
			strings.Join(activationIDs, ", "),
		)
	}
	return nil
}
