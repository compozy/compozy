package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// BuiltinTombstone records one permanently removed bundled extension name.
type BuiltinTombstone struct {
	Name      string
	RemovedAt time.Time
	RemovedBy string
}

// BuiltinTombstones is the durable store of permanently removed bundled names.
// The Registry implements it. A tombstone gates only bundled enrollment: it never
// changes a non-bundled row that happens to share the name.
// The interface is the package contract from the spec's Core Interfaces. Production removal and restore use the
// transactional and restore writers directly, so Tombstone and Tombstoned are consumed by contract tests and by
// future callers; they stay on the interface on purpose.
type BuiltinTombstones interface {
	// Tombstone records name as permanently removed and returns the stored record.
	Tombstone(ctx context.Context, name string, removedBy string) (BuiltinTombstone, error)
	// Clear removes the tombstone and reports whether one existed.
	Clear(ctx context.Context, name string) (bool, error)
	// Tombstoned reports whether bundled enrollment must be skipped for the name.
	Tombstoned(ctx context.Context, name string) (bool, error)
	// ListTombstones returns all tombstones in name order.
	ListTombstones(ctx context.Context) ([]BuiltinTombstone, error)
}

var _ BuiltinTombstones = (*Registry)(nil)

// insertBuiltinTombstoneSQL is shared by the standalone and the removal-transaction writers.
const insertBuiltinTombstoneSQL = `INSERT INTO extension_builtin_tombstones (name, removed_at, removed_by) VALUES (?, ?, ?)`

// normalizeBuiltinTombstoneInput returns the trimmed tombstone key and actor. Both are required.
func normalizeBuiltinTombstoneInput(name string, removedBy string) (string, string, error) {
	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return "", "", err
	}
	actor := strings.TrimSpace(removedBy)
	if actor == "" {
		return "", "", errors.New("extension: tombstone removed_by is required")
	}
	return trimmedName, actor, nil
}

// Tombstone records name as permanently removed. The name is stored trimmed.
// Permanent removal does not use this method: it writes the tombstone inside the row-delete transaction.
func (r *Registry) Tombstone(ctx context.Context, name string, removedBy string) (BuiltinTombstone, error) {
	if err := r.checkReady("tombstone built-in extension"); err != nil {
		return BuiltinTombstone{}, err
	}
	trimmedName, actor, err := normalizeBuiltinTombstoneInput(name, removedBy)
	if err != nil {
		return BuiltinTombstone{}, err
	}

	removedAt := r.now().UTC()
	if _, err := r.execWrite(
		ctx,
		"tombstone built-in extension",
		insertBuiltinTombstoneSQL,
		trimmedName,
		store.FormatTimestamp(removedAt),
		actor,
	); err != nil {
		return BuiltinTombstone{}, fmt.Errorf("extension: tombstone %q: %w", trimmedName, err)
	}
	r.invalidateEnabledBundledNames()
	return BuiltinTombstone{Name: trimmedName, RemovedAt: removedAt, RemovedBy: actor}, nil
}

// Clear removes the tombstone for name and reports whether one existed.
func (r *Registry) Clear(ctx context.Context, name string) (bool, error) {
	if err := r.checkReady("clear built-in tombstone"); err != nil {
		return false, err
	}
	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return false, err
	}
	result, err := r.execWrite(
		ctx,
		"clear built-in tombstone",
		`DELETE FROM extension_builtin_tombstones WHERE name = ?`,
		trimmedName,
	)
	if err != nil {
		return false, fmt.Errorf("extension: clear built-in tombstone %q: %w", trimmedName, err)
	}
	r.invalidateEnabledBundledNames()
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("extension: inspect built-in tombstone rows for %q: %w", trimmedName, err)
	}
	return rowsAffected > 0, nil
}

// Tombstoned reports whether a tombstone exists for name.
func (r *Registry) Tombstoned(ctx context.Context, name string) (bool, error) {
	if err := r.checkReady("read built-in tombstone"); err != nil {
		return false, err
	}
	trimmedName, err := normalizeExtensionName(name)
	if err != nil {
		return false, err
	}
	var exists bool
	if err := r.readDB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM extension_builtin_tombstones WHERE name = ?
		)
	`, trimmedName).Scan(&exists); err != nil {
		return false, fmt.Errorf("extension: read built-in tombstone %q: %w", trimmedName, err)
	}
	return exists, nil
}

// ListTombstones returns every tombstone ordered by name.
func (r *Registry) ListTombstones(ctx context.Context) (tombstones []BuiltinTombstone, err error) {
	if err := r.checkReady("list built-in tombstones"); err != nil {
		return nil, err
	}
	rows, err := r.readDB.QueryContext(ctx, `
		SELECT name, removed_at, removed_by
		FROM extension_builtin_tombstones
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("extension: list built-in tombstones: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("extension: close built-in tombstone rows: %w", closeErr))
		}
	}()

	tombstones = make([]BuiltinTombstone, 0)
	for rows.Next() {
		var name, removedAtText, removedBy string
		if scanErr := rows.Scan(&name, &removedAtText, &removedBy); scanErr != nil {
			return nil, fmt.Errorf("extension: scan built-in tombstone: %w", scanErr)
		}
		removedAt, parseErr := store.ParseTimestamp(removedAtText)
		if parseErr != nil {
			return nil, fmt.Errorf("extension: parse removed_at for built-in tombstone %q: %w", name, parseErr)
		}
		tombstones = append(tombstones, BuiltinTombstone{Name: name, RemovedAt: removedAt, RemovedBy: removedBy})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("extension: iterate built-in tombstones: %w", err)
	}
	return tombstones, nil
}
