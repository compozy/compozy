package daemon

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// RemovePermanently removes a bundled built-in and records its tombstone, so boot and update never
// re-enroll it until restore. Only operator sessions may call it.
func (s *daemonExtensionService) RemovePermanently(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	if err := validateExtensionPermanentRemovalActor(actor); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	return s.removeInstalledExtension(ctx, name, actor, "", true)
}

// removeExtensionPackage routes a removal to the permanent built-in coordinator, which writes the tombstone
// inside the row-delete transaction, or to the managed-removal coordinator. Both share the reload and commit
// compensation the caller supplies.
func (s *daemonExtensionService) removeExtensionPackage(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
	permanent bool,
	reload, commit func(context.Context) error,
) (extensionpkg.ManagedRemoveResult, error) {
	if permanent {
		return extensionpkg.RemoveBuiltinPermanently(
			ctx, s.homePaths, s.registry, name, extensionInstalledBy(actor), reload, commit,
		)
	}
	return extensionpkg.RemoveManagedExtension(ctx, s.homePaths, s.registry, name, reload, commit)
}

// applyBuiltinTombstone reports the tombstone a permanent removal recorded, so the payload carries when the
// built-in was removed and that restore can act on it.
func applyBuiltinTombstone(
	item *contract.ManagedExtensionRemovePayload,
	tombstone *extensionpkg.BuiltinTombstone,
) {
	if tombstone == nil {
		return
	}
	item.Permanent = true
	item.TombstonedAt = tombstone.RemovedAt.UTC().Format(time.RFC3339)
}
