package windowmanager

import (
	"context"
	"errors"
	"fmt"
)

type terminalRetirementCommand struct{ ids map[string]bool }

func (terminalRetirementCommand) CommandID() CommandID { return CommandWindowNavigate }

// ReconcileTerminalWindows removes dead terminal references, including undo and reopen history.
func ReconcileTerminalWindows(snapshot Snapshot, live func(string) bool) (Snapshot, bool) {
	return reconcileWindows(snapshot, func(window Window) bool {
		return window.App != "terminal" || window.InstanceKey == nil || live(*window.InstanceKey)
	})
}

// RetireTerminals removes matching bindings (all when ids is nil) without reversible close entries.
func (m *Manager) RetireTerminals(ctx context.Context, workspaceID WorkspaceID, ids map[string]bool) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		snapshot, err := m.Snapshot(ctx, workspaceID)
		if err != nil {
			return err
		}
		_, err = m.Execute(ctx, CommandRequest{
			WorkspaceID: workspaceID, ExpectedRevision: snapshot.Revision,
			Actor: Actor{Kind: systemActorKind, ID: "terminal.retire"}, Origin: "terminal.retire",
			Payload: terminalRetirementCommand{ids: ids},
		})
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrRevisionConflict) {
			return fmt.Errorf("retire terminal windows: %w", err)
		}
	}
}

func (r *reducer) retireTerminalWindows(snapshot *Snapshot, ids map[string]bool) bool {
	reconciled, changed := ReconcileTerminalWindows(*snapshot, func(id string) bool { return ids != nil && !ids[id] })
	if !changed {
		return false
	}
	for id, window := range snapshot.Windows {
		if _, exists := reconciled.Windows[id]; !exists {
			r.changes.window(id)
			r.changes.desktop(window.DesktopID)
		}
	}
	*snapshot = reconciled
	return true
}
