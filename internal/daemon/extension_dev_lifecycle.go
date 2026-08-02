package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
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
			rollbackErr = err
		}
	} else {
		_, rollbackErr = runtime.LinkDevelopmentFromOrigin(
			rollbackCtx,
			key.WorkspaceID,
			snapshot.OriginPath,
			snapshot.BundleGeneration,
		)
		if rollbackErr == nil {
			rollbackErr = s.registry.RestoreNetworkConfirmation(key, extensionpkg.NetworkConfirmation{
				Digest:      snapshot.NetworkRequirementDigest,
				ConfirmedBy: snapshot.NetworkConfirmedBy,
				ConfirmedAt: snapshot.NetworkConfirmedAt,
			})
		}
	}
	if rollbackErr == nil {
		rollbackErr = s.syncExtensionConsumers(rollbackCtx)
	}
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

func devCandidateConfirmationRequired(link *extensionpkg.DevLink, digest string) bool {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return false
	}
	return link == nil || strings.TrimSpace(link.NetworkRequirementDigest) != digest ||
		strings.TrimSpace(link.NetworkConfirmedBy) == "" || link.NetworkConfirmedAt.IsZero()
}

func (s *daemonExtensionService) confirmDevCandidateNetwork(
	key extensionpkg.InstanceKey,
	digest string,
	expectedDigest string,
	actor taskpkg.ActorContext,
) (*extensionpkg.NetworkConfirmation, error) {
	if strings.TrimSpace(expectedDigest) != strings.TrimSpace(digest) {
		return nil, &extensionpkg.NetworkConfirmationRequiredError{CurrentDigest: strings.TrimSpace(digest)}
	}
	confirmedBy, err := extensionNetworkConfirmationActor(actor)
	if err != nil {
		return nil, err
	}
	confirmedAt := s.now().UTC()
	if err := s.registry.ConfirmDevelopmentNetworkCandidate(key, digest, confirmedBy, confirmedAt); err != nil {
		return nil, err
	}
	return &extensionpkg.NetworkConfirmation{
		Digest: strings.TrimSpace(digest), ConfirmedBy: confirmedBy, ConfirmedAt: confirmedAt,
	}, nil
}
