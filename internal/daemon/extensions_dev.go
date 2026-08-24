package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) Dev(
	ctx context.Context,
	req contract.DevLinkExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ExtensionPayload{}, err
	}
	workspaceID, err := s.developmentWorkspaceID(ctx, actor)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	generation, err := runtime.InspectDevelopmentGeneration(ctx, workspaceID, req.OriginPath, req.GenerationHash)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	key := extensionpkg.InstanceKey{Name: generation.Name, WorkspaceID: workspaceID}
	var item contract.ExtensionPayload
	profile, err := s.extensionReadProfile(ctx, actor)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	err = s.lifecycle.withInstance(ctx, key, func() error {
		return s.linkDevelopmentExtension(ctx, runtime, key, generation, req, actor, profile, &item)
	})
	return item, err
}

func (s *daemonExtensionService) linkDevelopmentExtension(
	ctx context.Context,
	runtime extensionDevRuntime,
	key extensionpkg.InstanceKey,
	generation extensionpkg.DevelopmentGeneration,
	req contract.DevLinkExtensionRequest,
	actor taskpkg.ActorContext,
	profile extensionpkg.ProfileLens,
	item *contract.ExtensionPayload,
) error {
	snapshot, err := s.snapshotDevLink(key)
	if err != nil {
		return err
	}
	staged, err := runtime.StageDevelopmentLink(ctx, key, generation.OriginPath, generation.GenerationHash)
	if err != nil {
		return err
	}
	var confirmation *extensionpkg.NetworkConfirmation
	if devCandidateConfirmationRequired(staged, generation.NetworkRequirementDigest) {
		confirmation, err = s.confirmDevCandidateNetwork(
			key, generation.NetworkRequirementDigest, req.ConfirmNetworkDigest, actor,
		)
		if err != nil {
			return errors.Join(err, s.restoreStagedDevLink(key, snapshot))
		}
	}
	extension, err := runtime.ActivateDevelopmentLink(ctx, key)
	if err != nil {
		return s.rollbackDevLifecycle(ctx, runtime, key, snapshot, err)
	}
	if err := s.syncExtensionConsumers(ctx); err != nil {
		return s.rollbackDevLifecycle(ctx, runtime, key, snapshot, err)
	}
	*item, err = s.payloadFromExtension(ctx, extension, profile)
	if err != nil {
		return s.rollbackDevLifecycle(ctx, runtime, key, snapshot, err)
	}
	events := make([]extensionpkg.LifecycleEvent, 0, 2)
	if confirmation != nil {
		events = append(events, extensionpkg.LifecycleEvent{
			Type: eventspkg.ExtensionNetworkConfirmed, ExtensionName: key.Name,
			WorkspaceID: key.WorkspaceID, Digest: confirmation.Digest, ConfirmedBy: confirmation.ConfirmedBy,
		})
	}
	events = append(events, extensionpkg.LifecycleEvent{
		Type: eventspkg.ExtensionDevLinked, ExtensionName: item.Name,
		WorkspaceID: item.WorkspaceID, ExtensionGeneration: item.GenerationHash,
	})
	if err := s.recordCanonicalExtensionLifecycleEvents(ctx, actor, events...); err != nil {
		return s.rollbackDevLifecycle(ctx, runtime, key, snapshot, err)
	}
	return nil
}

func (s *daemonExtensionService) ReloadDev(
	ctx context.Context,
	name string,
	req contract.ReloadExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ExtensionPayload{}, err
	}
	workspaceID, err := s.developmentWorkspaceID(ctx, actor)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	key := extensionpkg.InstanceKey{Name: name, WorkspaceID: workspaceID}
	var item contract.ExtensionPayload
	err = s.lifecycle.withInstance(ctx, key, func() error {
		var reloadErr error
		item, reloadErr = s.reloadDevLocked(ctx, runtime, key, req, actor)
		return reloadErr
	})
	return item, err
}

func (s *daemonExtensionService) reloadDevLocked(
	ctx context.Context,
	runtime extensionDevRuntime,
	key extensionpkg.InstanceKey,
	req contract.ReloadExtensionRequest,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	snapshot, err := s.snapshotDevLink(key)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	if snapshot == nil {
		return contract.ExtensionPayload{}, fmt.Errorf("%w: %s", extensionpkg.ErrExtensionNotDevLinked, key.Name)
	}
	generation, confirmation, err := s.inspectDevReloadCandidate(ctx, runtime, key, snapshot, req, actor)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	return s.applyDevReload(ctx, runtime, key, snapshot, generation, confirmation, actor)
}

func (s *daemonExtensionService) inspectDevReloadCandidate(
	ctx context.Context,
	runtime extensionDevRuntime,
	key extensionpkg.InstanceKey,
	snapshot *extensionpkg.DevLink,
	req contract.ReloadExtensionRequest,
	actor taskpkg.ActorContext,
) (extensionpkg.DevelopmentGeneration, *extensionpkg.NetworkConfirmation, error) {
	generation, err := runtime.InspectDevelopmentGeneration(
		ctx,
		key.WorkspaceID,
		snapshot.OriginPath,
		req.GenerationHash,
	)
	if err != nil {
		return extensionpkg.DevelopmentGeneration{}, nil, err
	}
	if generation.Name != strings.TrimSpace(key.Name) {
		return extensionpkg.DevelopmentGeneration{}, nil, fmt.Errorf(
			"%w: generation manifest name %q does not match %q",
			extensionpkg.ErrExtensionGenerationInvalid,
			generation.Name,
			strings.TrimSpace(key.Name),
		)
	}
	if !devCandidateConfirmationRequired(snapshot, generation.NetworkRequirementDigest) {
		return generation, nil, nil
	}
	confirmation, err := s.confirmDevCandidateNetwork(
		key,
		generation.NetworkRequirementDigest,
		req.ConfirmNetworkDigest,
		actor,
	)
	if err != nil {
		return extensionpkg.DevelopmentGeneration{}, nil, err
	}
	return generation, confirmation, nil
}

func (s *daemonExtensionService) applyDevReload(
	ctx context.Context,
	runtime extensionDevRuntime,
	key extensionpkg.InstanceKey,
	snapshot *extensionpkg.DevLink,
	generation extensionpkg.DevelopmentGeneration,
	confirmation *extensionpkg.NetworkConfirmation,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	ext, err := runtime.ReloadExtension(ctx, key, generation.GenerationHash)
	if err != nil {
		eventErr := s.recordCanonicalExtensionLifecycleEvent(ctx, actor, extensionpkg.LifecycleEvent{
			Type: eventspkg.ExtensionReloadFailed, ExtensionName: key.Name,
			WorkspaceID: key.WorkspaceID, ExtensionGeneration: generation.GenerationHash,
		})
		return contract.ExtensionPayload{}, errors.Join(err, eventErr, s.restoreDevNetworkConfirmation(key, snapshot))
	}
	if err := s.syncExtensionConsumers(ctx); err != nil {
		return contract.ExtensionPayload{}, s.rollbackDevLifecycle(ctx, runtime, key, snapshot, err)
	}
	profile, err := s.extensionReadProfile(ctx, actor)
	if err != nil {
		return contract.ExtensionPayload{}, s.rollbackDevLifecycle(ctx, runtime, key, snapshot, err)
	}
	item, err := s.payloadFromExtension(ctx, ext, profile)
	if err != nil {
		return contract.ExtensionPayload{}, s.rollbackDevLifecycle(ctx, runtime, key, snapshot, err)
	}
	if err := s.recordDevReloadEvents(ctx, actor, key, item.GenerationHash, confirmation); err != nil {
		return contract.ExtensionPayload{}, s.rollbackDevLifecycle(ctx, runtime, key, snapshot, err)
	}
	s.evictExtensionMCPHealth(key.Name, key.WorkspaceID)
	return item, nil
}

func (s *daemonExtensionService) recordDevReloadEvents(
	ctx context.Context,
	actor taskpkg.ActorContext,
	key extensionpkg.InstanceKey,
	generationHash string,
	confirmation *extensionpkg.NetworkConfirmation,
) error {
	events := make([]extensionpkg.LifecycleEvent, 0, 2)
	if confirmation != nil {
		events = append(events, extensionpkg.LifecycleEvent{
			Type: eventspkg.ExtensionNetworkConfirmed, ExtensionName: key.Name,
			WorkspaceID: key.WorkspaceID, Digest: confirmation.Digest, ConfirmedBy: confirmation.ConfirmedBy,
		})
	}
	events = append(events, extensionpkg.LifecycleEvent{
		Type: eventspkg.ExtensionReloadCompleted, ExtensionName: key.Name,
		WorkspaceID: key.WorkspaceID, ExtensionGeneration: generationHash,
	})
	return s.recordCanonicalExtensionLifecycleEvents(ctx, actor, events...)
}

func (s *daemonExtensionService) ExtensionLogs(
	ctx context.Context,
	name string,
	after int64,
	streamEpoch string,
	actor taskpkg.ActorContext,
) (contract.ExtensionLogsResponse, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionLogsResponse{}, err
	}
	if err := ctx.Err(); err != nil {
		return contract.ExtensionLogsResponse{}, err
	}
	if err := actor.Validate(); err != nil {
		return contract.ExtensionLogsResponse{}, err
	}
	if !actor.Authority.Read {
		return contract.ExtensionLogsResponse{}, taskpkg.ErrPermissionDenied
	}
	workspaceID, err := s.scopedDevelopmentWorkspaceID(ctx, actor)
	if err != nil {
		return contract.ExtensionLogsResponse{}, err
	}
	profile, err := s.extensionReadProfile(ctx, actor)
	if err != nil {
		return contract.ExtensionLogsResponse{}, err
	}
	if !actor.Scope.Operator && workspaceID == "" {
		return contract.ExtensionLogsResponse{}, extensionpkg.ErrExtensionWorkspaceDenied
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return contract.ExtensionLogsResponse{}, err
	}
	snapshot, err := runtime.Logs(
		extensionpkg.ProfileInstanceKey(name, profile.ID, workspaceID),
		extensionpkg.ExtensionLogCursor{Sequence: after, StreamEpoch: strings.TrimSpace(streamEpoch)},
	)
	if err != nil {
		return contract.ExtensionLogsResponse{}, err
	}
	payloads := make([]contract.ExtensionLogPayload, 0, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		payloads = append(payloads, contract.ExtensionLogPayload{
			Sequence:       entry.Sequence,
			Timestamp:      entry.Timestamp,
			Message:        entry.Message,
			GenerationHash: entry.GenerationHash,
			StreamEpoch:    entry.StreamEpoch,
		})
	}
	return contract.ExtensionLogsResponse{Logs: payloads, StreamEpoch: snapshot.StreamEpoch}, nil
}

func (s *daemonExtensionService) ListScoped(
	ctx context.Context,
	actor taskpkg.ActorContext,
) ([]contract.ExtensionPayload, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if !actor.Authority.Read {
		return nil, taskpkg.ErrPermissionDenied
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return nil, err
	}
	workspaceID, err := s.scopedDevelopmentWorkspaceID(ctx, actor)
	if err != nil {
		return nil, err
	}
	profile, err := s.extensionReadProfile(ctx, actor)
	if err != nil {
		return nil, err
	}
	infos := runtime.ListForWorkspace(workspaceID)
	items := make([]contract.ExtensionPayload, 0, len(infos))
	for _, info := range infos {
		key := extensionpkg.InstanceKey{
			Name:        info.Name,
			WorkspaceID: workspaceID,
		}
		ext, getErr := s.projectExtensionReadProfile(ctx, runtime, key, profile)
		if getErr != nil {
			return nil, getErr
		}
		item, payloadErr := s.payloadFromExtension(ctx, ext, profile)
		if payloadErr != nil {
			return nil, payloadErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *daemonExtensionService) StatusScoped(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if err := ctx.Err(); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := actor.Validate(); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if !actor.Authority.Read {
		return contract.ExtensionPayload{}, taskpkg.ErrPermissionDenied
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	workspaceID, err := s.scopedDevelopmentWorkspaceID(ctx, actor)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	profile, err := s.extensionReadProfile(ctx, actor)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	ext, err := s.projectExtensionReadProfile(ctx, runtime, extensionpkg.InstanceKey{
		Name:        name,
		WorkspaceID: workspaceID,
	}, profile)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	return s.payloadFromExtension(ctx, ext, profile)
}

func (s *daemonExtensionService) RemoveScoped(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	if strings.TrimSpace(actor.Scope.WorkspaceID) == "" {
		return s.Remove(ctx, name, actor)
	}
	workspaceID, err := s.developmentWorkspaceID(ctx, actor)
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	key := extensionpkg.InstanceKey{Name: name, WorkspaceID: workspaceID}
	ext, getErr := runtime.GetForInstance(key)
	if getErr != nil {
		return contract.ManagedExtensionRemovePayload{}, getErr
	}
	if ext.Status.WorkspaceID == "" {
		if !actor.Scope.Operator {
			return contract.ManagedExtensionRemovePayload{}, extensionpkg.ErrExtensionWorkspaceDenied
		}
		return s.Remove(ctx, name, actor)
	}
	var item contract.ManagedExtensionRemovePayload
	err = s.lifecycle.withInstance(ctx, key, func() error {
		snapshot, snapshotErr := s.snapshotDevLink(key)
		if snapshotErr != nil {
			return snapshotErr
		}
		if snapshot == nil {
			return fmt.Errorf("%w: %s", extensionpkg.ErrExtensionNotDevLinked, name)
		}
		retirement, retireErr := s.retireExtensionSecretBindings(ctx, key)
		if retireErr != nil {
			return retireErr
		}
		if unlinkErr := runtime.UnlinkDevelopment(ctx, key); unlinkErr != nil {
			return errors.Join(unlinkErr, retirement.rollback(ctx, s))
		}
		if syncErr := s.syncExtensionConsumers(ctx); syncErr != nil {
			return s.rollbackDevRemoval(ctx, runtime, key, snapshot, retirement, syncErr)
		}
		item = contract.ManagedExtensionRemovePayload{
			Name: name, Path: snapshot.OriginPath, Status: "removed",
		}
		if eventErr := s.recordCanonicalExtensionLifecycleEvent(ctx, actor, extensionpkg.LifecycleEvent{
			Type: eventspkg.ExtensionDevUnlinked, ExtensionName: name,
			WorkspaceID: workspaceID, ExtensionGeneration: snapshot.BundleGeneration,
		}); eventErr != nil {
			return s.rollbackDevRemoval(ctx, runtime, key, snapshot, retirement, eventErr)
		}
		s.evictExtensionMCPHealth(key.Name, key.WorkspaceID)
		return nil
	})
	return item, err
}

func (s *daemonExtensionService) devRuntime() (extensionDevRuntime, error) {
	runtime, ok := s.runtime.(extensionDevRuntime)
	if !ok || runtime == nil {
		return nil, errors.New("daemon: extension development runtime is unavailable")
	}
	return runtime, nil
}

func (s *daemonExtensionService) developmentWorkspaceID(
	ctx context.Context,
	actor taskpkg.ActorContext,
) (string, error) {
	workspaceID, err := s.scopedDevelopmentWorkspaceID(ctx, actor)
	if err != nil {
		return "", err
	}
	if workspaceID == "" {
		return "", errors.New("daemon: a trusted workspace scope is required")
	}
	return workspaceID, nil
}

func (s *daemonExtensionService) scopedDevelopmentWorkspaceID(
	ctx context.Context,
	actor taskpkg.ActorContext,
) (string, error) {
	workspaceRef := strings.TrimSpace(actor.Scope.WorkspaceID)
	if workspaceRef == "" {
		return "", nil
	}
	if s.workspaceResolver == nil {
		return "", errors.New("daemon: workspace resolver is required for workspace-scoped extensions")
	}
	resolved, err := s.workspaceResolver.Resolve(ctx, workspaceRef)
	if err != nil {
		return "", fmt.Errorf("daemon: resolve extension workspace %q: %w", workspaceRef, err)
	}
	workspaceID := strings.TrimSpace(resolved.ID)
	if workspaceID == "" {
		return "", fmt.Errorf("daemon: resolved extension workspace %q has no stable registration ID", workspaceRef)
	}
	return workspaceID, nil
}
