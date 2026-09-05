package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func requireExpectedActiveTurn(session *Session, expectedTurnID string) (string, error) {
	if session == nil {
		return "", ErrPromptNotInProgress
	}
	session.mu.RLock()
	activeTurnID := strings.TrimSpace(session.currentTurnID)
	session.mu.RUnlock()
	if activeTurnID == "" {
		return "", ErrPromptNotInProgress
	}
	expected := strings.TrimSpace(expectedTurnID)
	if expected != "" && expected != activeTurnID {
		return "", &ActiveTurnMismatchError{ExpectedTurnID: expected, CurrentTurnID: activeTurnID}
	}
	return activeTurnID, nil
}

func (m *Manager) activateInterruptingInput(
	ctx context.Context,
	session *Session,
	entry *store.SessionInputQueueEntry,
) error {
	if session == nil {
		return errors.New("session: session is required")
	}
	if !session.IsPrompting() {
		m.startNextQueuedInputPrompt(session.ID)
		return nil
	}
	activeTurnID := strings.TrimSpace(session.CurrentTurnID())
	if activeTurnID != strings.TrimSpace(entry.TargetTurnID) {
		return fmt.Errorf(
			"%w: expected %s, active %s",
			ErrActiveTurnMismatch,
			entry.TargetTurnID,
			activeTurnID,
		)
	}
	if err := m.CancelTurn(ctx, session.ID, entry.TargetTurnID, CauseUserRequested); err != nil {
		return err
	}
	if !session.IsPrompting() {
		m.startNextQueuedInputPrompt(session.ID)
	}
	return nil
}

func (m *Manager) ensureInterruptingInputActivated(
	ctx context.Context,
	session *Session,
	entry *store.SessionInputQueueEntry,
) error {
	switch entry.Status {
	case store.SessionInputQueueStatusQueued:
		return m.activateInterruptingInput(ctx, session, entry)
	case store.SessionInputQueueStatusDispatching, store.SessionInputQueueStatusSent:
		return nil
	case store.SessionInputQueueStatusCanceled, store.SessionInputQueueStatusFailed:
		return fmt.Errorf(
			"%w: input %s is %s",
			store.ErrSessionPromptDispatchIndeterminate,
			entry.ID,
			entry.Status,
		)
	default:
		return fmt.Errorf("session: invalid interrupting input status %q", entry.Status)
	}
}

func (m *Manager) cleanupInterruptingInputActivationFailure(
	ctx context.Context,
	entry *store.SessionInputQueueEntry,
	cause error,
) error {
	if entry.Status != store.SessionInputQueueStatusQueued {
		return cause
	}
	return m.cancelUndeliverableInput(ctx, entry.SessionID, entry.ID, cause)
}

func (m *Manager) cancelUndeliverableInput(
	ctx context.Context,
	sessionID string,
	entryID string,
	cause error,
) error {
	_, cancelErr := m.inputQueue.Cancel(context.WithoutCancel(ctx), sessionID, entryID)
	if cancelErr != nil {
		return errors.Join(cause, fmt.Errorf("session: cancel undeliverable input: %w", cancelErr))
	}
	return cause
}
