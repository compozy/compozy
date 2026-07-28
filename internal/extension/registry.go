package extensionpkg

import (
	"database/sql"

	"errors"
	"fmt"

	"time"
)

var (
	// ErrExtensionNotFound reports that no installed extension matched the lookup.
	ErrExtensionNotFound = errors.New("extension: extension not found")
	// ErrExtensionExists reports that an extension name is already installed.
	ErrExtensionExists = errors.New("extension: extension already exists")
	// ErrExtensionChecksumMismatch reports that the provided checksum does not
	// match the on-disk extension artifact.
	ErrExtensionChecksumMismatch = errors.New("extension: checksum mismatch")
	// ErrExtensionHasActiveBundles reports that the extension lifecycle is
	// blocked by one or more active bundle activations.
	ErrExtensionHasActiveBundles = errors.New("extension: extension has active bundle activations")
)

const (
	registrySelectColumns = `
		SELECT
			name,
			version,
			source,
			enabled,
			manifest_path,
			installed_at,
			capabilities,
			actions,
			checksum,
			registry_slug,
			registry_name,
			remote_version,
			provenance_json
	`
	registryInsertColumns = `
			name,
			version,
			source,
			enabled,
			manifest_path,
			installed_at,
			capabilities,
			actions,
			checksum,
			registry_slug,
			registry_name,
			remote_version,
			provenance_json
	`
)

// NewRegistry constructs a registry over an existing SQLite connection.
func NewRegistry(db *sql.DB) *Registry {
	return &Registry{
		db: db,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// DB exposes the backing SQLite handle for composition-root integrations that
// need to build additional stores over the same registry database.
func (r *Registry) DB() *sql.DB {
	if r == nil {
		return nil
	}
	return r.db
}

// Install verifies the extension artifact checksum and persists the install as
// a user-sourced extension.
func (r *Registry) Install(manifest *Manifest, path string, checksum string, opts ...InstallOption) error {
	config := installConfig{
		source: SourceUser,
	}
	applyInstallOptions(&config, opts...)

	return r.installWithConfig(manifest, path, checksum, config)
}

// Uninstall removes one extension from the registry.
func (r *Registry) Uninstall(name string) error {
	if err := r.checkReady("uninstall extension"); err != nil {
		return err
	}

	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return err
	}
	if err := r.ensureNoActiveBundles(trimmedName); err != nil {
		return err
	}

	result, err := r.db.ExecContext(registryContext(), `DELETE FROM extensions WHERE name = ?`, trimmedName)
	if err != nil {
		return fmt.Errorf("extension: uninstall %q: %w", trimmedName, err)
	}

	if err := rowsAffectedNotFound(result, trimmedName); err != nil {
		return err
	}
	r.invalidateEnabledBundledNames()
	return nil
}

// Enable marks one installed extension as enabled.
func (r *Registry) Enable(name string) error {
	return r.updateEnabled(name, true)
}

// Disable marks one installed extension as disabled.
func (r *Registry) Disable(name string) error {
	return r.updateEnabled(name, false)
}

// List returns every installed extension ordered by name.
func (r *Registry) List() (extensions []ExtensionInfo, err error) {
	if err := r.checkReady("list extensions"); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(registryContext(), `
`+registrySelectColumns+`
		FROM extensions
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("extension: list extensions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("extension: close extension list rows: %w", closeErr))
		}
	}()

	extensions = make([]ExtensionInfo, 0)
	for rows.Next() {
		info, err := scanExtensionInfo(rows)
		if err != nil {
			return nil, err
		}
		extensions = append(extensions, *info)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("extension: iterate extensions: %w", err)
	}

	return extensions, nil
}

// Get returns one installed extension by name.
func (r *Registry) Get(name string) (*ExtensionInfo, error) {
	if err := r.checkReady("get extension"); err != nil {
		return nil, err
	}

	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return nil, err
	}

	row := r.db.QueryRowContext(registryContext(), `
`+registrySelectColumns+`
		FROM extensions
		WHERE name = ?
	`, trimmedName)

	info, err := scanExtensionInfo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &ExtensionNotFoundError{Name: trimmedName}
	}
	if err != nil {
		return nil, err
	}

	return info, nil
}
