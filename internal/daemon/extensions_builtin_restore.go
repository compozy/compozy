package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const extensionRestoreStatusRestored = "restored"

// RestoreBuiltin clears a built-in's tombstone and enrolls it at its bundled version, enabled, under the
// package lock permanent removal uses. Restore needs write authority like other extension writes, so agents
// with that authority can restore (Q5 a).
func (s *daemonExtensionService) RestoreBuiltin(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionRestorePayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionRestorePayload{}, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ExtensionRestorePayload{}, err
	}
	trimmedName := strings.TrimSpace(name)
	var item contract.ExtensionRestorePayload
	err := s.lifecycle.withPackageMutation(ctx, []string{trimmedName}, func() error {
		var restoreErr error
		item, restoreErr = s.restoreBuiltinLocked(ctx, trimmedName)
		return restoreErr
	})
	return item, err
}

// restoreBuiltinLocked runs the restore order: tombstone check, operator-row check, tombstone clear,
// shared enrollment, reload. A failure after the clear rolls back to the removed state.
func (s *daemonExtensionService) restoreBuiltinLocked(
	ctx context.Context,
	name string,
) (contract.ExtensionRestorePayload, error) {
	builtin, known := s.builtinEnrollmentFor(name)
	tombstone, tombstoned, err := s.registry.BuiltinTombstoneFor(ctx, name)
	if err != nil {
		return contract.ExtensionRestorePayload{}, err
	}
	if !tombstoned {
		if !known {
			return contract.ExtensionRestorePayload{}, &extensionpkg.ExtensionNotFoundError{Name: name}
		}
		return contract.ExtensionRestorePayload{}, fmt.Errorf(
			"daemon: restore %q: %w", name, extensionpkg.ErrExtensionBuiltinNotRemoved,
		)
	}
	existing, getErr := s.registry.Get(name)
	switch {
	case getErr == nil && existing.Source != extensionpkg.SourceBundled:
		return contract.ExtensionRestorePayload{}, fmt.Errorf(
			"daemon: restore %q: %w", name, extensionpkg.ErrExtensionExists,
		)
	case getErr != nil && !errors.Is(getErr, extensionpkg.ErrExtensionNotFound):
		return contract.ExtensionRestorePayload{}, getErr
	}
	if !known {
		return contract.ExtensionRestorePayload{}, fmt.Errorf(
			"daemon: restore %q: no bundled enrollment: %w", name, extensionpkg.ErrExtensionRestoreFailed,
		)
	}
	if _, err := s.registry.Clear(ctx, name); err != nil {
		return contract.ExtensionRestorePayload{}, fmt.Errorf("daemon: restore %q: clear tombstone: %w", name, err)
	}
	if err := builtin.Enroll(s.homePaths, s.registry); err != nil {
		return contract.ExtensionRestorePayload{}, s.rollbackBuiltinRestore(ctx, name, tombstone, err)
	}
	if err := s.reloadExtensions(ctx, name); err != nil {
		return contract.ExtensionRestorePayload{}, s.rollbackBuiltinRestore(ctx, name, tombstone, err)
	}
	payload, err := s.restoredBuiltinPayload(name)
	if err != nil {
		return contract.ExtensionRestorePayload{}, s.rollbackBuiltinRestore(ctx, name, tombstone, err)
	}
	s.logger.Info("extension.builtin.restored", "name", name)
	return payload, nil
}

// restoredBuiltinPayload reads back the enrolled row. It reports the bundled version, enablement and path.
func (s *daemonExtensionService) restoredBuiltinPayload(name string) (contract.ExtensionRestorePayload, error) {
	installed, err := s.registry.Get(name)
	if err != nil {
		return contract.ExtensionRestorePayload{}, fmt.Errorf("daemon: read restored %q: %w", name, err)
	}
	path, err := extensionpkg.ManagedInstallPathChecked(s.homePaths, name)
	if err != nil {
		return contract.ExtensionRestorePayload{}, fmt.Errorf("daemon: resolve restored %q path: %w", name, err)
	}
	return contract.ExtensionRestorePayload{
		Name:    name,
		Status:  extensionRestoreStatusRestored,
		Source:  installed.Source.String(),
		Version: installed.Version,
		Enabled: installed.Enabled,
		Path:    path,
	}, nil
}

// rollbackBuiltinRestore returns the registry to the removed state after a failed restore. It removes a
// partially enrolled bundled row, reloads so the runtime matches the registry, and writes the tombstone back
// exactly as it was read. The log carries the name only, because enrollment errors can contain paths.
func (s *daemonExtensionService) rollbackBuiltinRestore(
	ctx context.Context,
	name string,
	tombstone extensionpkg.BuiltinTombstone,
	cause error,
) error {
	var rollbackErr error
	if _, getErr := s.registry.Get(name); getErr == nil {
		noopReload := func(context.Context) error { return nil }
		noopCommit := func(context.Context) error { return nil }
		if _, removeErr := extensionpkg.RemoveManagedExtension(
			ctx, s.homePaths, s.registry, name, noopReload, noopCommit,
		); removeErr != nil {
			rollbackErr = errors.Join(rollbackErr, removeErr)
		}
		rollbackErr = errors.Join(rollbackErr, s.reloadExtensions(ctx, name))
	}
	if err := s.registry.RestoreBuiltinTombstone(ctx, tombstone); err != nil {
		rollbackErr = errors.Join(rollbackErr, err)
	}
	s.logger.Error("extension.builtin.restore_failed", "name", name, "rolled_back", rollbackErr == nil)
	return errors.Join(extensionpkg.ErrExtensionRestoreFailed, cause, rollbackErr)
}

// builtinEnrollmentFor returns the shared enrollment for a known built-in name.
func (s *daemonExtensionService) builtinEnrollmentFor(name string) (bundledEnrollment, bool) {
	table := s.builtinEnrollments
	if table == nil {
		table = bundledEnrollments
	}
	for _, entry := range table {
		if entry.Name == name {
			return entry, true
		}
	}
	return bundledEnrollment{}, false
}
