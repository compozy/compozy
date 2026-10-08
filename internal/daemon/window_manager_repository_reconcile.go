package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/cmdpalette/corecmds"
	"github.com/compozy/compozy/internal/windowmanager"
)

func (r *windowManagerRepository) reconcileLoadedSnapshot(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
	snapshot windowmanager.Snapshot,
	entry clientstate.Entry,
) (windowmanager.Snapshot, error) {
	migrated := storedWindowManagerSnapshotVersion(entry.Value) == windowmanager.LegacySnapshotVersion
	reconciled, changed := windowmanager.ReconcileRegisteredApps(snapshot, corecmds.RegisteredApp)
	if !changed {
		if migrated {
			return snapshot, r.persistMigratedSnapshot(ctx, workspaceID, snapshot, entry.Rev)
		}
		return snapshot, nil
	}
	if !migrated {
		revision, err := windowmanager.NextTopologyRevision(snapshot.Revision)
		if err != nil {
			return windowmanager.Snapshot{}, fmt.Errorf("daemon: reconciled snapshot revision: %w", err)
		}
		reconciled.Revision = revision
	}
	if err := windowmanager.ValidateSnapshot(reconciled); err != nil {
		return windowmanager.Snapshot{}, fmt.Errorf("daemon: validate reconciled snapshot: %w", err)
	}
	encoded, err := json.Marshal(reconciled)
	if err != nil {
		return windowmanager.Snapshot{}, fmt.Errorf("daemon: encode reconciled snapshot: %w", err)
	}
	_, err = r.service.Apply(ctx, clientstate.WorkspaceID(workspaceID), windowManagerStateDomain,
		[]clientstate.Op{{Kind: clientstate.OpPut, Key: windowManagerSnapshotKey(r.profileID),
			Value: encoded, IfRev: entry.Rev}},
		clientstate.ApplyOptions{Origin: "window-manager.snapshot.reconcile"})
	if err != nil {
		return windowmanager.Snapshot{}, mapWindowManagerStoreError("persist reconciled snapshot", err)
	}
	droppedApps, rewrittenRoutes := windowManagerReconciliationAudit(snapshot)
	r.logger.Info("windowmanager.snapshot_reconciled", "workspace_id", workspaceID,
		"profile_id", r.profileID, "dropped_apps", droppedApps, "rewritten_routes", rewrittenRoutes,
		"revision", reconciled.Revision)
	return reconciled, nil
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
