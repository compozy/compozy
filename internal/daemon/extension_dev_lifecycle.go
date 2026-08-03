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
		rollbackErr = errors.Join(rollbackErr, s.restoreDevNetworkConfirmation(key, snapshot))
		_, err = runtime.ActivateDevelopmentLink(rollbackCtx, key)
		rollbackErr = errors.Join(rollbackErr, err)
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

func (s *daemonExtensionService) restoreStagedDevLink(
	key extensionpkg.InstanceKey,
	snapshot *extensionpkg.DevLink,
) error {
	if snapshot == nil {
		err := s.registry.UnlinkDev(key.Name, key.WorkspaceID)
		if errors.Is(err, extensionpkg.ErrExtensionNotDevLinked) {
			return nil
		}
		return err
	}
	if _, err := s.registry.LinkDev(extensionpkg.DevLinkRequest{
		Name:                     snapshot.ExtensionName,
		WorkspaceID:              snapshot.WorkspaceID,
		OriginPath:               snapshot.OriginPath,
		GenerationHash:           snapshot.BundleGeneration,
		NetworkRequirementDigest: snapshot.NetworkRequirementDigest,
	}); err != nil {
		return err
	}
	return s.restoreDevNetworkConfirmation(key, snapshot)
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
		s.rollbackDevLifecycle(ctx, runtime, key, snapshot, cause),
		retirementErr,
	)
}

func (s *daemonExtensionService) restoreDevNetworkConfirmation(
	key extensionpkg.InstanceKey,
	snapshot *extensionpkg.DevLink,
) error {
	if snapshot == nil {
		return nil
	}
	return s.registry.RestoreNetworkConfirmation(key, extensionpkg.NetworkConfirmation{
		Digest:      snapshot.NetworkRequirementDigest,
		ConfirmedBy: snapshot.NetworkConfirmedBy,
		ConfirmedAt: snapshot.NetworkConfirmedAt,
	})
}
