package windowmanager

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// workspaceUnregisteredCommand is deliberately private: workspace unregistration is a
// lifecycle reconciliation, not a client-facing window-manager command.
type workspaceUnregisteredCommand struct {
	binding string
}

func (workspaceUnregisteredCommand) CommandID() CommandID { return CommandWindowNavigate }

// ReconcileUnregisteredWorkspace retires bindings and publishes the partition event.
func (m *Manager) ReconcileUnregisteredWorkspace(
	ctx context.Context,
	workspaceID WorkspaceID,
	binding string,
) error {
	if m == nil {
		return errors.New("window manager is required")
	}
	if ctx == nil {
		return errors.New("window manager workspace reconciliation context is required")
	}
	target := strings.TrimSpace(binding)
	if target == "" {
		return fmt.Errorf("window manager workspace reconciliation id is required: %w", ErrInvalidCommand)
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		snapshot, err := m.Snapshot(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("load workspace %q for unregistered workspace %q: %w", workspaceID, target, err)
		}
		_, err = m.Execute(ctx, CommandRequest{
			WorkspaceID:      workspaceID,
			ExpectedRevision: snapshot.Revision,
			Actor:            Actor{Kind: systemActorKind, ID: "workspace.unregister"},
			Origin:           "workspace.unregister",
			Payload:          workspaceUnregisteredCommand{binding: target},
		})
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrRevisionConflict) {
			return fmt.Errorf("reconcile unregistered workspace %q in workspace %q: %w", target, workspaceID, err)
		}
	}
}
