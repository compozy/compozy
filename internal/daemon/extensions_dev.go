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
	workspaceID, err := developmentWorkspaceID(actor)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	ext, err := runtime.LinkDevelopmentFromOrigin(
		ctx,
		workspaceID,
		req.OriginPath,
		req.GenerationHash,
	)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.syncExtensionConsumers(ctx); err != nil {
		return contract.ExtensionPayload{}, err
	}
	item := s.payloadFromExtension(ext)
	if err := s.recordExtensionEvent(ctx, eventspkg.ExtensionDevLinked, actor, item); err != nil {
		return contract.ExtensionPayload{}, err
	}
	return item, nil
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
	workspaceID, err := developmentWorkspaceID(actor)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	ext, err := runtime.ReloadExtension(
		ctx,
		extensionpkg.InstanceKey{Name: name, WorkspaceID: workspaceID},
		req.GenerationHash,
	)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.syncExtensionConsumers(ctx); err != nil {
		return contract.ExtensionPayload{}, err
	}
	item := s.payloadFromExtension(ext)
	if err := s.recordExtensionEvent(ctx, eventspkg.ExtensionReloaded, actor, item); err != nil {
		return contract.ExtensionPayload{}, err
	}
	return item, nil
}

func (s *daemonExtensionService) ExtensionLogs(
	ctx context.Context,
	name string,
	after int64,
	actor taskpkg.ActorContext,
) ([]contract.ExtensionLogPayload, error) {
	if err := s.checkReady(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if !actor.Authority.Read {
		return nil, taskpkg.ErrPermissionDenied
	}
	workspaceID := strings.TrimSpace(actor.Scope.WorkspaceID)
	if !actor.Scope.Operator && workspaceID == "" {
		return nil, extensionpkg.ErrExtensionWorkspaceDenied
	}
	runtime, err := s.devRuntime()
	if err != nil {
		return nil, err
	}
	entries, err := runtime.Logs(
		extensionpkg.InstanceKey{Name: name, WorkspaceID: workspaceID},
		after,
	)
	if err != nil {
		return nil, err
	}
	payloads := make([]contract.ExtensionLogPayload, 0, len(entries))
	for _, entry := range entries {
		payloads = append(payloads, contract.ExtensionLogPayload{
			Sequence:       entry.Sequence,
			Timestamp:      entry.Timestamp,
			Message:        entry.Message,
			GenerationHash: entry.GenerationHash,
		})
	}
	return payloads, nil
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
	workspaceID := strings.TrimSpace(actor.Scope.WorkspaceID)
	infos := runtime.ListForWorkspace(workspaceID)
	items := make([]contract.ExtensionPayload, 0, len(infos))
	for _, info := range infos {
		ext, getErr := runtime.GetForInstance(extensionpkg.InstanceKey{
			Name:        info.Name,
			WorkspaceID: workspaceID,
		})
		if getErr != nil {
			return nil, getErr
		}
		items = append(items, s.payloadFromExtension(ext))
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
	ext, err := runtime.GetForInstance(extensionpkg.InstanceKey{
		Name:        name,
		WorkspaceID: strings.TrimSpace(actor.Scope.WorkspaceID),
	})
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	return s.payloadFromExtension(ext), nil
}

func (s *daemonExtensionService) RemoveScoped(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	if err := validateExtensionWriteActor(actor); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	workspaceID := strings.TrimSpace(actor.Scope.WorkspaceID)
	if workspaceID == "" {
		return s.Remove(ctx, name, actor)
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
	path := ""
	if ext.DevLink != nil {
		path = ext.DevLink.OriginPath
	}
	if err := runtime.UnlinkDevelopment(ctx, key); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	if err := s.syncExtensionConsumers(ctx); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	item := contract.ManagedExtensionRemovePayload{Name: name, Path: path, Status: "removed"}
	if err := s.recordExtensionLifecycleEvent(
		ctx,
		eventspkg.ExtensionDevUnlinked,
		actor,
		extensionLifecycleEventPayload{Name: name, WorkspaceID: workspaceID, Status: item.Status},
	); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	return item, nil
}

func (s *daemonExtensionService) devRuntime() (extensionDevRuntime, error) {
	runtime, ok := s.runtime.(extensionDevRuntime)
	if !ok || runtime == nil {
		return nil, errors.New("daemon: extension development runtime is unavailable")
	}
	return runtime, nil
}

func developmentWorkspaceID(actor taskpkg.ActorContext) (string, error) {
	workspaceID := strings.TrimSpace(actor.Scope.WorkspaceID)
	if workspaceID == "" {
		return "", errors.New("daemon: a trusted workspace scope is required")
	}
	return workspaceID, nil
}
