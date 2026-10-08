package daemon

import (
	"context"
	"encoding/json"
	"fmt"

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
	r.logger.Info("windowmanager.snapshot_reconciled", "workspace_id", workspaceID,
		"revision", reconciled.Revision)
	return reconciled, nil
}
