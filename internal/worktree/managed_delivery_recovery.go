package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// RecoverManagedDeliveries re-admits durable intents after runtime recovery.
// A mismatch remains an explicit failed operation; recovery never broadens the
// recorded caller, branch, base, remote, selected paths, or expected commit.
func (s *Service) RecoverManagedDeliveries(ctx context.Context) error {
	if s.root == "" || s.deliverySessions == nil {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(s.root, ".delivery"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		journal, err := readDeliveryJournal(filepath.Join(s.root, ".delivery", entry.Name()))
		if err != nil {
			return fmt.Errorf("worktree: read managed delivery journal: %w", err)
		}
		if journal == nil {
			continue
		}
		operation := ExitOperation{
			ID:          journal.OperationID,
			ProfileID:   journal.Item.ProfileID,
			WorkspaceID: journal.Item.WorkspaceID,
			WorktreeID:  journal.Item.ID,
			Action:      string(ExitActionDeliver),
		}
		state, event := "failed", EventExitActionFailed
		message := "Managed delivery interrupted by daemon restart; reconciling durable intent."
		switch journal.Phase {
		case exitStepCompleted:
			state, event, message = exitStepCompleted, EventExitActionCompleted, ""
		case exitOperationCanceled:
			state, event, message = exitOperationCanceled, EventExitActionCanceled, "Managed delivery canceled."
		}
		if _, err := s.finishExitOperation(
			ctx,
			operation,
			state,
			event,
			ExitEventPayload{
				OperationID: operation.ID,
				Action:      ExitActionDeliver,
				State:       state,
				Result:      &journal.Result,
				Message:     message,
			},
		); err != nil {
			return err
		}
		if journal.Phase == exitStepCompleted || journal.Phase == exitOperationCanceled {
			continue
		}
		if _, err := s.SubmitManagedDelivery(
			ctx,
			journal.Item.WorkspaceID,
			journal.Item.ID,
			journal.SessionID,
			journal.Request,
		); err != nil {
			s.logger.ErrorContext(
				ctx,
				"managed delivery recovery refused",
				"worktree_id",
				journal.Item.ID,
				"error",
				err,
			)
		}
	}
	return nil
}
