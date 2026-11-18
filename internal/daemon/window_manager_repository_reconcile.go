package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/cmdpalette/corecmds"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/compozy/compozy/internal/windowmanager/statekeys"
)

func (r *windowManagerRepository) reconcileLoadedSnapshot(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
	snapshot windowmanager.Snapshot,
	entry clientstate.Entry,
) (windowmanager.Snapshot, error) {
	// Decoding already applied any snapshot-version migration, so reconciliation
	// sees retired app ids only after they were rewritten to their replacements.
	reconciled, changed := windowmanager.ReconcileRegisteredApps(snapshot, corecmds.RegisteredApp)
	pending, err := r.emptyDesktopsPending(ctx, workspaceID)
	if err != nil {
		return windowmanager.Snapshot{}, err
	}
	retired := false
	if pending {
		reconciled, retired = windowmanager.RetireEmptyDesktopPlaceholders(reconciled)
	}
	var ops []clientstate.Op
	if changed || retired {
		revision, err := windowmanager.NextTopologyRevision(snapshot.Revision)
		if err != nil {
			return windowmanager.Snapshot{}, fmt.Errorf("reconciled snapshot revision: %w", err)
		}
		reconciled.Revision = revision
		if err := windowmanager.ValidateSnapshot(reconciled); err != nil {
			return windowmanager.Snapshot{}, fmt.Errorf("daemon: validate reconciled snapshot: %w", err)
		}
		encoded, err := json.Marshal(reconciled)
		if err != nil {
			return windowmanager.Snapshot{}, fmt.Errorf("daemon: encode reconciled snapshot: %w", err)
		}
		ops = append(ops, clientstate.Op{Kind: clientstate.OpPut, Key: windowManagerSnapshotKey(r.profileID),
			Value: encoded, IfRev: entry.Rev})
	}
	if pending {
		ops = append(ops, r.emptyDesktopsMarker())
	}
	if len(ops) == 0 {
		return snapshot, nil
	}
	if _, err := r.service.Apply(ctx, clientstate.WorkspaceID(workspaceID), windowManagerStateDomain, ops,
		clientstate.ApplyOptions{Origin: "window-manager.snapshot.reconcile"}); err != nil {
		return windowmanager.Snapshot{}, mapWindowManagerStoreError("persist reconciled snapshot", err)
	}
	r.emptyDesktopsSettled.Store(workspaceID, struct{}{})
	if !changed && !retired {
		return snapshot, nil
	}
	droppedApps, rewrittenRoutes := windowManagerReconciliationAudit(snapshot)
	r.logger.Info("windowmanager.snapshot_reconciled", "workspace_id", workspaceID,
		"profile_id", r.profileID, "dropped_apps", droppedApps, "rewritten_routes", rewrittenRoutes,
		"retired_placeholders", retired, "revision", reconciled.Revision)
	return reconciled, nil
}

// emptyDesktopsPending reports whether this profile's aggregate in the workspace may still hold the
// New Tabs that releases up to v0.7.4 seeded into every empty desktop (#1799). The retirement runs
// once per stored aggregate: its durable marker is written with the first load or commit after the
// upgrade, so a New Tab someone opens later on an empty desktop is never mistaken for a seeded one.
// Remove the marker and RetireEmptyDesktopPlaceholders in v0.9.0.
func (r *windowManagerRepository) emptyDesktopsPending(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
) (bool, error) {
	if _, settled := r.emptyDesktopsSettled.Load(workspaceID); settled {
		return false, nil
	}
	_, err := r.service.Get(ctx, clientstate.WorkspaceID(workspaceID), windowManagerStateDomain,
		statekeys.EmptyDesktops(r.profileID))
	if errors.Is(err, clientstate.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, mapWindowManagerStoreError("read empty-desktop marker", err)
	}
	r.emptyDesktopsSettled.Store(workspaceID, struct{}{})
	return false, nil
}

func (r *windowManagerRepository) emptyDesktopsMarker() clientstate.Op {
	return clientstate.Op{Kind: clientstate.OpPut, Key: statekeys.EmptyDesktops(r.profileID),
		Value: []byte(`{"issue":1799}`)}
}

func windowManagerReconciliationAudit(snapshot windowmanager.Snapshot) ([]string, int) {
	dropped := make(map[string]bool)
	rewritten := 0
	inspect := func(window windowmanager.Window) {
		if !corecmds.RegisteredApp(window.App) {
			dropped[window.App] = true
			return
		}
		if window.App != "settings" {
			return
		}
		if window.Route.Pathname == "/settings/memory" {
			rewritten++
		}
		for _, route := range window.NavStack {
			if route.Pathname == "/settings/memory" {
				rewritten++
			}
		}
	}
	for _, window := range snapshot.Windows {
		inspect(window)
	}
	for _, entries := range [][]windowmanager.HistoryEntry{snapshot.History.Undo, snapshot.History.Redo} {
		for _, entry := range entries {
			for _, state := range []windowmanager.State{entry.Before, entry.After} {
				for _, window := range state.Windows {
					inspect(window)
				}
			}
		}
	}
	for _, entry := range snapshot.ClosedEntries {
		for _, window := range entry.Windows {
			inspect(window)
		}
	}
	apps := make([]string, 0, len(dropped))
	for app := range dropped {
		apps = append(apps, app)
	}
	slices.Sort(apps)
	return apps, rewritten
}
