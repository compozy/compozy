package windowmanager

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// sessionDeletionCommand is deliberately private: session deletion is a
// lifecycle reconciliation, not a client-facing window-manager command.
type sessionDeletionCommand struct {
	sessionID string
}

func (sessionDeletionCommand) CommandID() CommandID { return CommandWindowNavigate }

func isSessionDeletionCommand(command Command) bool {
	_, ok := command.(sessionDeletionCommand)
	return ok
}

// ReconcileDeletedSession retires every durable window for one deleted
// session onto the empty session route and publishes the workspace event.
func (m *Manager) ReconcileDeletedSession(
	ctx context.Context,
	workspaceID WorkspaceID,
	sessionID string,
) error {
	if m == nil {
		return errors.New("window manager is required")
	}
	if ctx == nil {
		return errors.New("window manager session reconciliation context is required")
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return fmt.Errorf("window manager session reconciliation id is required: %w", ErrInvalidCommand)
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		snapshot, err := m.Snapshot(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("load workspace %q for deleted session %q: %w", workspaceID, target, err)
		}
		_, err = m.Execute(ctx, CommandRequest{
			WorkspaceID:      workspaceID,
			ExpectedRevision: snapshot.Revision,
			Actor:            Actor{Kind: "system", ID: "session.delete"},
			Origin:           "session.delete",
			Payload:          sessionDeletionCommand{sessionID: target},
		})
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrRevisionConflict) {
			return fmt.Errorf("reconcile deleted session %q in workspace %q: %w", target, workspaceID, err)
		}
	}
}
