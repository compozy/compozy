package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/windowmanager"
)

func (r *windowManagerRegistry) partitionWorkspaceIDs(ctx context.Context) ([]clientstate.WorkspaceID, error) {
	if enumerator, ok := r.engine.(interface {
		PartitionIDs(context.Context) ([]clientstate.WorkspaceID, error)
	}); ok {
		return enumerator.PartitionIDs(ctx)
	}
	workspaces, err := r.workspaces.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := []clientstate.WorkspaceID{clientstate.WorkspaceID(windowmanager.GlobalDesktopWorkspaceID), clientstate.WorkspaceID(windowmanager.UserDesktopWorkspaceID)}
	for _, workspace := range workspaces {
		ids = append(ids, clientstate.WorkspaceID(workspace.ID))
	}
	return ids, nil
}

func (r *windowManagerRegistry) reconcilePartitions(ctx context.Context, reconcile func(*windowmanager.Manager, windowmanager.WorkspaceID) error) error {
	partitions, err := r.desktopPartitions(ctx, "")
	if err != nil {
		return err
	}
	for _, partition := range partitions {
		manager, err := r.For(partition.profileID)
		if errors.Is(err, errWindowManagerProfileDeleted) {
			continue
		}
		if err != nil {
			return err
		}
		if err := reconcile(manager, windowmanager.WorkspaceID(partition.workspaceID)); err != nil && !errors.Is(err, windowmanager.ErrWorkspaceNotFound) {
			return fmt.Errorf("daemon: reconcile profile %q partition %q: %w", partition.profileID, partition.workspaceID, err)
		}
	}
	return nil
}

func (r *windowManagerRegistry) ReconcileUnregisteredWorkspace(ctx context.Context, workspaceID string) error {
	return r.reconcilePartitions(ctx, func(manager *windowmanager.Manager, partition windowmanager.WorkspaceID) error {
		return manager.ReconcileUnregisteredWorkspace(ctx, partition, workspaceID)
	})
}

func (r *windowManagerRegistry) ReconcileStaleBindings(ctx context.Context) error {
	return r.reconcilePartitions(ctx, func(manager *windowmanager.Manager, partition windowmanager.WorkspaceID) error {
		snapshot, err := manager.Snapshot(ctx, partition)
		if err != nil {
			return err
		}
		bindings := map[string]struct{}{}
		collect := func(window windowmanager.Window) {
			routes := append([]windowmanager.RouteIntent{window.Route}, window.NavStack...)
			for _, route := range routes {
				var binding string
				if json.Unmarshal(route.Search["workspace"], &binding) == nil && binding != "" && binding != "@user" {
					bindings[binding] = struct{}{}
				}
			}
		}
		for _, window := range snapshot.Windows {
			collect(window)
		}
		for _, entry := range snapshot.ClosedEntries {
			for _, window := range entry.Windows {
				collect(window)
			}
		}
		for _, entries := range [][]windowmanager.HistoryEntry{snapshot.History.Undo, snapshot.History.Redo} {
			for _, entry := range entries {
				for _, state := range []windowmanager.State{entry.Before, entry.After} {
					for _, window := range state.Windows {
						collect(window)
					}
				}
			}
		}
		ids := make([]string, 0, len(bindings))
		for id := range bindings {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			err := r.authorizer.ResolveWorkspace(ctx, windowmanager.WorkspaceID(id))
			if err == nil {
				continue
			}
			if !errors.Is(err, windowmanager.ErrWorkspaceNotFound) {
				return err
			}
			if err := manager.ReconcileUnregisteredWorkspace(ctx, partition, id); err != nil {
				return err
			}
		}
		return nil
	})
}
