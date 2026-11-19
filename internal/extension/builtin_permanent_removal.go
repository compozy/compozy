package extensionpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

// builtinCompensationTimeout bounds the detached work that returns a failed built-in removal to its removed state.
// It matches the 15-second rollback budget of rollbackManagedRemoval, so both compensation paths share one lifetime.
const builtinCompensationTimeout = 15 * time.Second

// ErrExtensionNotBuiltin reports that permanent removal targeted a row that is not bundled.
var ErrExtensionNotBuiltin = errors.New("extension: extension is not a built-in extension")

// ErrExtensionBuiltinAttached reports that a built-in still has other installations, so a
// removal would only detach one scope and could not record a tombstone for the built-in.
var ErrExtensionBuiltinAttached = errors.New("extension: built-in extension has other installations")

// ErrExtensionBuiltinRemovalCompensationFailed reports that a permanent removal committed its tombstone and its
// compensation failed. The built-in stays removed, and restore brings it back.
var ErrExtensionBuiltinRemovalCompensationFailed = errors.New(
	"extension: built-in removal was committed and could not be fully compensated",
)

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
// A later failure restores the row and directory, then clears the tombstone once the row is back. If that
// compensation fails, the built-in returns to its removed state (no row, no install directory, tombstone recorded),
// and the error wraps ErrExtensionBuiltinRemovalCompensationFailed so the caller can name the restore path.
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
	installDir, err := InstalledExtensionDir(info)
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
		if !errors.Is(err, ErrExtensionRemovalCompensationFailed) {
			return ManagedRemoveResult{}, err
		}
		keepErr := routed.keepRemovedAfterFailedCompensation(ctx, installDir, reload)
		return ManagedRemoveResult{}, fmt.Errorf(
			"%w: %q keeps its tombstone; run `compozy extension restore %s` to bring it back: %w",
			ErrExtensionBuiltinRemovalCompensationFailed, trimmedName, trimmedName, errors.Join(err, keepErr),
		)
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
// and clears that tombstone when compensation reinstalls the row.
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

// Install is the compensation path that restores the row. The tombstone clears here, before the reload that follows:
// a tombstoned name grants no tool trust, and the reload publishes the restored row. When a later compensation step
// fails, keepRemovedAfterFailedCompensation returns the built-in to its removed state.
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

// keepRemovedAfterFailedCompensation returns a failed compensation to the removed state: no row, no install
// directory, and the tombstone recorded. It then reloads the runtime so it serves the removed state the registry
// records, not the restored row. It runs detached from the caller's cancellation, like the other rollbacks, and it
// reloads even when the cleanup failed, because the runtime must follow whatever state the registry now holds.
func (p *builtinPermanentRegistry) keepRemovedAfterFailedCompensation(
	ctx context.Context,
	installDir string,
	reload MutationReload,
) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), builtinCompensationTimeout)
	defer cancel()
	cleanupErr := p.cleanRemovedRecord(cleanupCtx, installDir)
	return errors.Join(cleanupErr, reconcileRuntime(cleanupCtx, reload))
}

// cleanRemovedRecord deletes a row that the compensation reinstalled, then records the tombstone and removes the
// install directory. A compensation can leave the directory behind even when no row was reinstalled: the directory
// rollback runs before the reinstall, so a reinstall that fails leaves the restored directory on disk.
func (p *builtinPermanentRegistry) cleanRemovedRecord(ctx context.Context, installDir string) error {
	_, getErr := p.registry.Get(p.name)
	if getErr != nil && !errors.Is(getErr, ErrExtensionNotFound) {
		return getErr
	}
	if getErr == nil {
		// The row must be gone before the tombstone returns: a failed uninstall stops here, so a row never sits beside it.
		if err := p.RemovalRegistry.Uninstall(p.name); err != nil {
			return err
		}
	}
	return errors.Join(p.ensureTombstone(ctx), removeRestoredInstallDir(installDir))
}

// reconcileRuntime reloads the name so the runtime matches the registry after the cleanup. A nil reload skips the
// step, as it does for RemoveManagedExtension.
func reconcileRuntime(ctx context.Context, reload MutationReload) error {
	if reload == nil {
		return nil
	}
	return reload(ctx)
}

// ensureTombstone writes the removal's tombstone back when it is not recorded, from the record the removal committed.
func (p *builtinPermanentRegistry) ensureTombstone(ctx context.Context) error {
	tombstoned, err := p.registry.Tombstoned(ctx, p.name)
	if err != nil || tombstoned {
		return err
	}
	if p.tombstone == nil {
		return fmt.Errorf("extension: built-in tombstone for %q was never recorded", p.name)
	}
	return p.registry.RestoreBuiltinTombstone(ctx, *p.tombstone)
}

// removeRestoredInstallDir removes the install directory that a compensation reinstall put back.
func removeRestoredInstallDir(installDir string) error {
	change, err := stageExtensionDirRemoval(installDir)
	if err != nil {
		return err
	}
	return change.Commit()
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
