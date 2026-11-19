package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const extensionRestoreStatusRestored = "restored"

// builtinRestoreRollbackTimeout bounds the rollback of a failed restore. It matches the 15-second rollback budget of
// rollbackManagedRemoval, the other compensation path; no shared constant exists for it.
const builtinRestoreRollbackTimeout = 15 * time.Second

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
		item, restoreErr = s.restoreBuiltinLocked(ctx, trimmedName, actor)
		return restoreErr
	})
	return item, err
}

// ListRemovedBuiltins returns every permanently removed built-in, in name order. The listing is read-only
// and needs no actor: it reports tombstones that restore can act on.
func (s *daemonExtensionService) ListRemovedBuiltins(
	ctx context.Context,
) ([]contract.ExtensionRemovedBuiltinPayload, error) {
	if err := s.checkReady(); err != nil {
		return nil, err
	}
	tombstones, err := s.registry.ListTombstones(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]contract.ExtensionRemovedBuiltinPayload, 0, len(tombstones))
	for _, tombstone := range tombstones {
		items = append(items, contract.ExtensionRemovedBuiltinPayload{
			Name:      tombstone.Name,
			RemovedAt: tombstone.RemovedAt.UTC().Format(time.RFC3339),
			RemovedBy: tombstone.RemovedBy,
		})
	}
	return items, nil
}

// restoreBuiltinLocked runs the restore order: tombstone check, operator-row check, tombstone clear,
// shared enrollment, default-profile enablement, reload. A failure after the clear rolls back to the removed
// state, unless a partially enrolled row cannot be removed; see rollbackBuiltinRestore.
func (s *daemonExtensionService) restoreBuiltinLocked(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
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
	// Enrollment keeps a disabled default-profile exception that the row already holds: a dev-linked name keeps its
	// exceptions when its row is deleted. Restore enables the built-in through the enablement step SetEnablement uses.
	// SetEnablement itself takes the name lock restore already holds, so restore calls the unlocked step.
	profile := extensionDefaultProfileLens()
	if err := s.applyProfileEnablement(ctx, name, profile.ID, profile.Name, true, actor); err != nil {
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
// exactly as it was read, but only once no row remains for the name. When the partial row cannot be removed,
// the row stays and the tombstone stays clear, because a row and a tombstone must never coexist for one name.
// The log carries the name only, because enrollment errors can contain paths.
func (s *daemonExtensionService) rollbackBuiltinRestore(
	ctx context.Context,
	name string,
	tombstone extensionpkg.BuiltinTombstone,
	cause error,
) error {
	// The caller may already be canceled, so the rollback runs detached with its own budget. The tombstone write
	// gets a budget of its own: a slow removal or reload must not leave the built-in with neither a row nor a tombstone.
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.restoreRollbackBudget())
	defer cancel()
	var rollbackErr error
	partialRowGone := false
	_, getErr := s.registry.Get(name)
	switch {
	case errors.Is(getErr, extensionpkg.ErrExtensionNotFound):
		partialRowGone = true
	case getErr == nil:
		noopReload := func(context.Context) error { return nil }
		noopCommit := func(context.Context) error { return nil }
		_, removeErr := extensionpkg.RemoveManagedExtension(
			rollbackCtx, s.homePaths, s.registry, name, noopReload, noopCommit,
		)
		partialRowGone = removeErr == nil
		rollbackErr = errors.Join(rollbackErr, removeErr)
		rollbackErr = errors.Join(rollbackErr, s.reloadExtensions(rollbackCtx, name))
	default:
		// The row state is unknown, so the tombstone stays clear rather than risk a row beside it.
		rollbackErr = errors.Join(rollbackErr, getErr)
	}
	if partialRowGone {
		if err := s.writeTombstoneBack(ctx, tombstone); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	s.logger.Error(
		"extension.builtin.restore_failed",
		"name", name,
		"rolled_back", partialRowGone && rollbackErr == nil,
	)
	return errors.Join(extensionpkg.ErrExtensionRestoreFailed, cause, rollbackErr)
}

// writeTombstoneBack records the tombstone a failed restore cleared, on a detached budget of its own. The rollback
// budget covers the removal and reload that precede it; a reload that spends it must not expire this write.
func (s *daemonExtensionService) writeTombstoneBack(
	ctx context.Context,
	tombstone extensionpkg.BuiltinTombstone,
) error {
	tombstoneCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), builtinRestoreRollbackTimeout)
	defer cancel()
	return s.registry.RestoreBuiltinTombstone(tombstoneCtx, tombstone)
}

// restoreRollbackBudget returns the budget of the rollback's removal and reload. Tests shorten it through
// restoreRollbackTimeout; production always uses builtinRestoreRollbackTimeout.
func (s *daemonExtensionService) restoreRollbackBudget() time.Duration {
	if s.restoreRollbackTimeout > 0 {
		return s.restoreRollbackTimeout
	}
	return builtinRestoreRollbackTimeout
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
