package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// Runs inside the lifecycle instance lock. Staging is already validated; no installed files change before Prepare.
func (s *daemonExtensionService) commitPreparedInstallWithInputs(
	ctx context.Context,
	prepared preparedDaemonExtensionInstall,
	req contract.InstallExtensionRequest,
	confirmation *extensionpkg.NetworkConfirmation,
	actor taskpkg.ActorContext,
	event extensionpkg.LifecycleEvent,
	item *contract.ExtensionPayload,
) error {
	binder := extensionInputBinder{service: s}
	plan, err := binder.Prepare(
		ctx,
		extensionpkg.GlobalInstanceKey(prepared.name),
		extensionDefaultProfileLens(),
		prepared.manifest,
		req.Inputs,
	)
	if err != nil {
		return err
	}
	allocations, err := s.prepareInstallMCPAllocations(ctx, prepared)
	if err != nil {
		return err
	}
	if err := s.commitPreparedInstall(ctx, prepared, confirmation, actor, event, item, plan); err != nil {
		return errors.Join(err, binder.Rollback(ctx, plan), s.rollbackInstallMCPAllocations(ctx, allocations))
	}
	return nil
}
