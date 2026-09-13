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
		event := extensionpkg.LifecycleEvent{
			Type: eventspkg.ExtensionDevUnlinked, ExtensionName: name,
			WorkspaceID: workspaceID, ExtensionGeneration: snapshot.BundleGeneration,
		}
		if err := s.commitDevMCPRetirement(ctx, actor, key, event); err != nil {
			return s.rollbackDevRemoval(ctx, runtime, key, snapshot, retirement, err)
		}
		s.evictExtensionMCPHealth(key.Name, key.WorkspaceID)
		return nil
	})
	return item, err
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
