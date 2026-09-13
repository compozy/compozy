package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonExtensionService) RemoveScoped(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
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
	var item contract.ManagedExtensionRemovePayload
	err = s.lifecycle.withPackageMutation(ctx, []string{name}, func() error {
		var err error
		item, err = s.removeScopedExtensionLocked(ctx, name, actor, workspaceID)
		return err
	})
	return item, err
}

func (s *daemonExtensionService) removeScopedExtensionLocked(
	ctx context.Context, name string, actor taskpkg.ActorContext, workspaceID string,
) (contract.ManagedExtensionRemovePayload, error) {
	link, err := s.snapshotDevLink(extensionpkg.InstanceKey{Name: name, WorkspaceID: workspaceID})
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	if link == nil {
		return s.removeInstalledExtensionLocked(ctx, name, actor, workspaceID)
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
		return s.removeInstalledExtensionLocked(ctx, name, actor, "")
	}
	snapshot := link
	retirement, retireErr := s.retireExtensionSecretBindings(ctx, key)
	if retireErr != nil {
		return contract.ManagedExtensionRemovePayload{}, retireErr
	}
	if unlinkErr := runtime.UnlinkDevelopment(ctx, key); unlinkErr != nil {
		return contract.ManagedExtensionRemovePayload{}, errors.Join(unlinkErr, retirement.rollback(ctx, s))
	}
	if syncErr := s.syncExtensionConsumers(ctx); syncErr != nil {
		return contract.ManagedExtensionRemovePayload{}, s.rollbackDevRemoval(
			ctx,
			runtime,
			key,
			snapshot,
			retirement,
			syncErr,
		)
	}
	item := contract.ManagedExtensionRemovePayload{
		Name: name, Path: snapshot.OriginPath, Status: extensionRemovalStatusRemoved,
	}
	event := extensionpkg.LifecycleEvent{
		Type: eventspkg.ExtensionDevUnlinked, ExtensionName: name,
		WorkspaceID: workspaceID, ExtensionGeneration: snapshot.BundleGeneration,
	}
	if err := s.commitDevMCPRetirement(ctx, actor, key, event); err != nil {
		return contract.ManagedExtensionRemovePayload{}, s.rollbackDevRemoval(
			ctx,
			runtime,
			key,
			snapshot,
			retirement,
			err,
		)
	}
	s.evictExtensionMCPHealth(key.Name, key.WorkspaceID)
	return item, nil
}

func (s *daemonExtensionService) commitDevMCPRetirement(
	ctx context.Context, actor taskpkg.ActorContext, key extensionpkg.InstanceKey, event extensionpkg.LifecycleEvent,
) error {
	if s.mcpAllocations == nil {
		return s.recordCanonicalExtensionLifecycleEvent(ctx, actor, event)
	}
	summary, err := s.canonicalLifecycleEventSink(actor).summary(ctx, event)
	if err != nil {
		return err
	}
	return s.mcpAllocations.RetireWorkspace(ctx, key.Name, key.WorkspaceID, summary)
}
