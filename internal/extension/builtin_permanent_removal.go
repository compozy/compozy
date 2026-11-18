package extensionpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

// ErrExtensionNotBuiltin reports that permanent removal targeted a row that is not bundled.
var ErrExtensionNotBuiltin = errors.New("extension: extension is not a built-in extension")

// ErrExtensionBuiltinAttached reports that a built-in still has other installations, so a
// removal would only detach one scope and could not record a tombstone for the built-in.
var ErrExtensionBuiltinAttached = errors.New("extension: built-in extension has other installations")

// ExtensionNotBuiltinError names the source of a row that permanent removal refused.
type ExtensionNotBuiltinError struct {
	Name   string
	Source string
}

// Error describes the refused source without changing any state.
func (e *ExtensionNotBuiltinError) Error() string {
	return fmt.Sprintf(
		"extension: %q has source %q; only built-in extensions can be removed permanently",
		e.Name,
		e.Source,
	)
}

// Is matches ErrExtensionNotBuiltin.
func (e *ExtensionNotBuiltinError) Is(target error) bool {
	return target == ErrExtensionNotBuiltin
}

// RequireBuiltinForPermanentRemoval refuses any row whose source is not bundled.
func RequireBuiltinForPermanentRemoval(info *ExtensionInfo) error {
	if info == nil {
		return errors.New("extension: permanent removal requires a registry row")
	}
	if info.Source != SourceBundled {
		return &ExtensionNotBuiltinError{Name: info.Name, Source: info.Source.String()}
	}
	return nil
}

// RemoveBuiltinPermanently removes one bundled extension exactly as RemoveManagedExtension does, and
// records its tombstone in the same registry transaction as the row delete (Safety Invariant 1).
// A later failure restores the row and directory, then clears the tombstone once the row is back.
func RemoveBuiltinPermanently(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	registry *Registry,
	name string,
	removedBy string,
	reload MutationReload,
	commit ManagedRemovalCommit,
) (ManagedRemoveResult, error) {
	if registry == nil {
		return ManagedRemoveResult{}, errors.New("extension: registry is required")
	}
	info, err := registry.Get(name)
	if err != nil {
		return ManagedRemoveResult{}, err
	}
	if err := RequireBuiltinForPermanentRemoval(info); err != nil {
		return ManagedRemoveResult{}, err
	}
	trimmedName, actor, err := normalizeBuiltinTombstoneInput(info.Name, removedBy)
	if err != nil {
		return ManagedRemoveResult{}, err
	}

	routed := &builtinPermanentRegistry{
		RemovalRegistry: registry,
		registry:        registry,
		name:            trimmedName,
		removedBy:       actor,
	}
	result, err := removeManagedExtensionWithDataOps(
		ctx,
		homePaths,
		routed,
		trimmedName,
		reload,
		defaultExtensionDataRemovalOps(),
		commit,
	)
	if err != nil {
		return ManagedRemoveResult{}, err
	}
	if routed.tombstone == nil {
		return ManagedRemoveResult{}, fmt.Errorf(
			"extension: permanent removal of %q committed no tombstone",
			trimmedName,
		)
	}
	result.Tombstone = routed.tombstone
	slog.Info("extension.builtin.tombstoned", "name", trimmedName, "removed_by", actor)
	return result, nil
}

// builtinPermanentRegistry routes the coordinator's row step through the transactional tombstone write
// and clears that tombstone only when compensation has put the row back.
type builtinPermanentRegistry struct {
	RemovalRegistry
	registry  *Registry
	name      string
	removedBy string
	// tombstone is the record the row-delete transaction committed; it is nil until Uninstall succeeds.
	tombstone *BuiltinTombstone
}

// Uninstall deletes the bundled row and inserts its tombstone in one transaction.
func (p *builtinPermanentRegistry) Uninstall(name string) error {
	if name != p.name {
		return fmt.Errorf("extension: permanent removal uninstall %q does not match %q", name, p.name)
	}
	tombstone, err := p.registry.uninstallBuiltinTombstoned(p.name, p.removedBy)
	if err != nil {
		return err
	}
	p.tombstone = &tombstone
	return nil
}

// Install is the compensation path that restores the row. The tombstone clears only after the row is back.
func (p *builtinPermanentRegistry) Install(
	manifest *Manifest,
	path string,
	checksum string,
	opts ...InstallOption,
) error {
	if err := p.RemovalRegistry.Install(manifest, path, checksum, opts...); err != nil {
		return err
	}
	if _, err := p.registry.Clear(registryContext(), p.name); err != nil {
		return fmt.Errorf("extension: clear built-in tombstone after restoring %q: %w", p.name, err)
	}
	return nil
}

// uninstallBuiltinTombstoned deletes a bundled row and inserts its tombstone in one write transaction.
// Any failure, including a conflicting tombstone, rolls the row delete back.
func (r *Registry) uninstallBuiltinTombstoned(name string, removedBy string) (BuiltinTombstone, error) {
	if err := r.checkReady("permanently remove built-in extension"); err != nil {
		return BuiltinTombstone{}, err
	}
	trimmedName, actor, err := normalizeBuiltinTombstoneInput(name, removedBy)
	if err != nil {
		return BuiltinTombstone{}, err
	}

	removedAt := r.now().UTC()
	err = store.ExecuteWriteOperation(
		registryContext(),
		r.db,
		"permanently remove built-in extension",
		func(ctx context.Context, tx *store.WriteTx) error {
			var source string
			if err := tx.QueryRowContext(
				ctx,
				`SELECT source FROM extensions WHERE name = ?`,
				trimmedName,
			).Scan(&source); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return &ExtensionNotFoundError{Name: trimmedName}
				}
				return err
			}
			if source != SourceBundled.String() {
				return &ExtensionNotBuiltinError{Name: trimmedName, Source: source}
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM extensions WHERE name = ?`, trimmedName); err != nil {
				return err
			}
			_, err := tx.ExecContext(
				ctx,
				insertBuiltinTombstoneSQL,
				trimmedName,
				store.FormatTimestamp(removedAt),
				actor,
			)
			return err
		},
	)
	if err != nil {
		return BuiltinTombstone{}, fmt.Errorf("extension: permanently remove built-in %q: %w", trimmedName, err)
	}
	r.invalidateEnabledBundledNames()
	return BuiltinTombstone{Name: trimmedName, RemovedAt: removedAt, RemovedBy: actor}, nil
}
