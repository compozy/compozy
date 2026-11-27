package daemon

import (
	"context"
	"errors"
	"fmt"

	extensionpkg "github.com/compozy/compozy/internal/extension"
)

func (s *daemonExtensionService) snapshotDevLink(key extensionpkg.InstanceKey) (*extensionpkg.DevLink, error) {
	link, err := s.registry.GetDevLink(key.Name, key.WorkspaceID)
	if errors.Is(err, extensionpkg.ErrExtensionNotDevLinked) {
		return nil, nil
	}
	return link, err
}

func (s *daemonExtensionService) rollbackDevLifecycle(
	ctx context.Context,
	runtime extensionDevRuntime,
	key extensionpkg.InstanceKey,
	snapshot *extensionpkg.DevLink,
	allocations *extensionMCPAllocationSnapshot,
	cause error,
) error {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extensionLifecycleRollbackTimeout)
	defer cancel()
	var rollbackErr error
	if snapshot == nil {
		if err := runtime.UnlinkDevelopment(rollbackCtx, key); err != nil &&
			!errors.Is(err, extensionpkg.ErrExtensionNotDevLinked) {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	} else {
		_, err := runtime.StageDevelopmentLink(
			rollbackCtx,
			key,
			snapshot.OriginPath,
			snapshot.BundleGeneration,
		)
		rollbackErr = errors.Join(rollbackErr, err)
		_, err = runtime.ActivateDevelopmentLink(rollbackCtx, key)
		rollbackErr = errors.Join(rollbackErr, err)
	}
	if rollbackErr == nil {
		rollbackErr = s.rollbackExtensionMCPAllocations(rollbackCtx, allocations)
	}
	rollbackErr = errors.Join(rollbackErr, s.syncExtensionConsumers(rollbackCtx))
	if rollbackErr != nil {
		rollbackErr = fmt.Errorf(
			"daemon: rollback development extension lifecycle %q in workspace %q: %w",
			key.Name,
			key.WorkspaceID,
			rollbackErr,
		)
	}
	return errors.Join(cause, rollbackErr)
}

func (s *daemonExtensionService) rollbackDevRemoval(
	ctx context.Context,
	runtime extensionDevRuntime,
	key extensionpkg.InstanceKey,
	snapshot *extensionpkg.DevLink,
	retirement *extensionSecretRetirement,
	cause error,
) error {
	retirementErr := retirement.rollback(ctx, s)
	return errors.Join(
		s.rollbackDevLifecycle(ctx, runtime, key, snapshot, nil, cause),
		retirementErr,
	)
}
