package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// ErrExtensionBuiltinNotRemoved reports that restore targeted a built-in with no tombstone.
var ErrExtensionBuiltinNotRemoved = errors.New("extension: built-in extension is not permanently removed")

// ErrExtensionRestoreFailed reports that restoring a built-in failed. Unless a partially enrolled row cannot be
// removed, the tombstone is written back before this error is returned, so the built-in stays removed.
var ErrExtensionRestoreFailed = errors.New("extension: restore built-in extension failed")

// BuiltinTombstoneFor returns the tombstone recorded for name. The boolean is false when none exists.
func (r *Registry) BuiltinTombstoneFor(ctx context.Context, name string) (BuiltinTombstone, bool, error) {
	trimmedName := strings.TrimSpace(name)
	tombstones, err := r.ListTombstones(ctx)
	if err != nil {
		return BuiltinTombstone{}, false, err
	}
	for _, tombstone := range tombstones {
		if tombstone.Name == trimmedName {
			return tombstone, true, nil
		}
	}
	return BuiltinTombstone{}, false, nil
}

// RestoreBuiltinTombstone writes a tombstone back exactly as it was read, including its original
// removal time and actor. Restore uses it when enrollment fails after the tombstone was cleared.
func (r *Registry) RestoreBuiltinTombstone(ctx context.Context, tombstone BuiltinTombstone) error {
	if err := r.checkReady("restore built-in tombstone"); err != nil {
		return err
	}
	trimmedName, actor, err := normalizeBuiltinTombstoneInput(tombstone.Name, tombstone.RemovedBy)
	if err != nil {
		return err
	}
	if _, err := r.execWrite(
		ctx,
		"restore built-in tombstone",
		insertBuiltinTombstoneSQL,
		trimmedName,
		store.FormatTimestamp(tombstone.RemovedAt),
		actor,
	); err != nil {
		return fmt.Errorf("extension: restore built-in tombstone %q: %w", trimmedName, err)
	}
	r.invalidateEnabledBundledNames()
	return nil
}
