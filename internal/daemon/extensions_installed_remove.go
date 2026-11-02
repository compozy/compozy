package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensionmcp"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const extensionRemovalStatusRemoved = "removed"

// removeInstalledExtension holds the package lock across attachment selection and compensation.
func (s *daemonExtensionService) removeInstalledExtension(
	ctx context.Context, name string, actor taskpkg.ActorContext, workspaceID string,
) (contract.ManagedExtensionRemovePayload, error) {
	var item contract.ManagedExtensionRemovePayload
	err := s.lifecycle.withPackageMutation(ctx, []string{name}, func() error {
		var err error
		item, err = s.removeInstalledExtensionLocked(ctx, name, actor, workspaceID)
		return err
	})
	return item, err
}

// Missing attachments authorize orphan recovery only with explicit operator-global intent.
func (s *daemonExtensionService) removeInstalledExtensionLocked(
	ctx context.Context, name string, actor taskpkg.ActorContext, workspaceID string,
) (contract.ManagedExtensionRemovePayload, error) {
	profileID := actor.ReadScope.ProfileID
	if profileID == "" {
		profileID = store.DefaultProfileID
	}
	attachments, err := s.registry.Installations(ctx, name)
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	if len(attachments) == 0 && actor.Scope.Operator && workspaceID == "" {
		return s.removeManagedExtensionLocked(ctx, name, actor, extensionpkg.GlobalInstanceKey(name), true)
	}
	selected, err := s.registry.ResolveInstallation(ctx, name, extensionpkg.InstallationScope{
		ProfileID: profileID, WorkspaceID: workspaceID,
	})
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	if !actor.Scope.Operator && (selected.Scope.ProfileID != profileID ||
		selected.Scope.WorkspaceID != actor.Scope.WorkspaceID) {
		return contract.ManagedExtensionRemovePayload{}, extensionpkg.ErrExtensionWorkspaceDenied
	}
	key := extensionpkg.InstanceKey{
		Name:        name,
		ProfileID:   selected.Scope.ProfileID,
		WorkspaceID: selected.Scope.WorkspaceID,
	}
	if len(attachments) == 1 {
		return s.removeManagedExtensionLocked(ctx, name, actor, key, false)
	}
	return s.detachInstalledExtension(ctx, name, actor, selected.Scope)
}

// Detaching one scope keeps the shared package and compensates failed publication.
func (s *daemonExtensionService) detachInstalledExtension(
	ctx context.Context, name string, actor taskpkg.ActorContext, scope extensionpkg.InstallationScope,
) (contract.ManagedExtensionRemovePayload, error) {
	info, err := s.registry.Get(name)
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	path, err := extensionpkg.InstalledExtensionDir(info)
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	snapshot, err := s.registry.SnapshotRemovalState(ctx, name)
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	allocations, err := s.snapshotMCPAllocations(
		ctx,
		extensionpkg.GlobalInstanceKey(name),
		extensionMCPPackageAllocations,
	)
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	if err := s.registry.DetachInstallation(ctx, name, scope); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	if err := s.reloadExtensions(ctx, name); err != nil {
		return contract.ManagedExtensionRemovePayload{}, s.rollbackInstalledDetach(
			ctx,
			name,
			snapshot,
			allocations,
			err,
		)
	}
	if err := s.retireDetachedMCPAllocations(ctx, name, actor, scope.WorkspaceID); err != nil {
		return contract.ManagedExtensionRemovePayload{}, s.rollbackInstalledDetach(
			ctx,
			name,
			snapshot,
			allocations,
			err,
		)
	}
	s.evictExtensionMCPHealth(name, scope.WorkspaceID)
	return contract.ManagedExtensionRemovePayload{Name: name, Path: path, Status: extensionRemovalStatusRemoved},
		s.paletteNotifier.NotifyExtensionChanged(ctx, scope.WorkspaceID, name)
}

// retireDetachedMCPAllocations keeps reservations still reachable through another attachment.
func (s *daemonExtensionService) retireDetachedMCPAllocations(
	ctx context.Context, name string, actor taskpkg.ActorContext, workspaceID string,
) error {
	summary, err := s.extensionRemovalSummary(actor, name, workspaceID)
	if err != nil {
		return err
	}
	if s.mcpAllocations == nil {
		if s.eventWriter == nil {
			return nil
		}
		return s.eventWriter.WriteEventSummary(ctx, summary)
	}
	records, err := s.mcpAllocations.ListAll(ctx)
	if err != nil {
		return err
	}
	var retired []extensionmcp.Target
	for _, record := range records {
		if record.Extension != name {
			continue
		}
		installation, err := s.registry.ResolveInstallation(ctx, name, extensionpkg.InstallationScope{
			ProfileID: record.ProfileID, WorkspaceID: record.WorkspaceID,
		})
		if err != nil && !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			return err
		}
		if err != nil || installation.Scope.WorkspaceID != record.WorkspaceID {
			retired = append(retired, record.Target)
		}
	}
	return s.mcpAllocations.RetireTargets(ctx, retired, summary)
}

// Attachment restoration precedes runtime publication and MCP allocation restoration.
func (s *daemonExtensionService) rollbackInstalledDetach(
	ctx context.Context, name string, snapshot extensionpkg.RemovalState,
	allocations *extensionMCPAllocationSnapshot, cause error,
) error {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extensionLifecycleRollbackTimeout)
	defer cancel()
	if err := s.registry.RestoreRemovalState(rollbackCtx, name, snapshot); err != nil {
		return errors.Join(cause, err)
	}
	if err := s.reloadExtensions(rollbackCtx, name); err != nil {
		return errors.Join(cause, err)
	}
	return errors.Join(cause, s.rollbackExtensionMCPAllocations(rollbackCtx, allocations))
}

// Orphan removal retires all package reservations atomically rather than only the global cell.
func (s *daemonExtensionService) deleteRemovedExtensionMCPAllocations(
	ctx context.Context,
	name string,
	key extensionpkg.InstanceKey,
	orphan bool,
) error {
	if s.mcpAllocations == nil {
		return nil
	}
	if !orphan {
		return s.mcpAllocations.DeleteWorkspace(ctx, name, key.WorkspaceID)
	}
	records, err := s.mcpAllocations.ListAll(ctx)
	if err != nil {
		return err
	}
	var targets []extensionmcp.Target
	for _, record := range records {
		if record.Extension == name {
			targets = append(targets, record.Target)
		}
	}
	return s.mcpAllocations.DeleteTargets(ctx, targets)
}
