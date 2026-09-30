package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/diagnostics"
)

// RecoverManagedDeliveries repairs receipts without replaying unknown or terminal intents.
func (s *Service) RecoverManagedDeliveries(ctx context.Context) error {
	journals := s.readManagedDeliveryJournals(ctx)
	running, err := s.store.ListRunningExitOperations(ctx)
	if err != nil {
		return err
	}
	for _, operation := range running {
		if ExitAction(operation.Action) != ExitActionDeliver {
			continue
		}
		if matchingDeliveryJournal(journals, operation) != nil {
			continue
		}
		item, err := s.store.Get(ctx, operation.WorkspaceID, operation.WorktreeID)
		if err != nil {
			return err
		}
		if item != nil {
			operation.ProfileID = item.ProfileID
		}
		if _, err := s.finishExitOperation(ctx, operation, exitStepFailed, EventExitActionFailed, ExitEventPayload{
			OperationID: operation.ID, Action: ExitActionDeliver, State: exitStepFailed,
			Message: "Managed delivery journal unavailable; interrupted operation cannot be recovered.",
		}); err != nil {
			return err
		}
	}
	for _, journal := range journals {
		if err := s.finishManagedDeliveryReceipt(ctx, journal); err != nil {
			return err
		}
		if deliveryJournalTerminal(journal) || s.deliverySessions == nil {
			continue
		}
		if _, err := s.SubmitManagedDelivery(ctx, journal.Item.WorkspaceID, journal.Item.ID,
			journal.SessionID, journal.Request); err != nil {
			s.logger.ErrorContext(ctx, "managed delivery recovery refused",
				"worktree_id", journal.Item.ID, "error", err)
		}
	}
	return nil
}

func (s *Service) readManagedDeliveryJournals(ctx context.Context) []*managedDeliveryJournal {
	if s.root == "" {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(s.root, ".delivery"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.logger.ErrorContext(ctx, "managed delivery journals unavailable", "error", err)
		}
		return nil
	}
	journals := make([]*managedDeliveryJournal, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		journal, err := readDeliveryJournal(filepath.Join(s.root, ".delivery", entry.Name()))
		if err != nil {
			s.logger.ErrorContext(ctx, "managed delivery journal unreadable", "file", entry.Name(), "error", err)
			continue
		}
		if journal != nil {
			journals = append(journals, journal)
		}
	}
	return journals
}

func matchingDeliveryJournal(journals []*managedDeliveryJournal, operation ExitOperation) *managedDeliveryJournal {
	for _, journal := range journals {
		if journal.OperationID == operation.ID && journal.Item.ID == operation.WorktreeID &&
			journal.Item.WorkspaceID == operation.WorkspaceID {
			return journal
		}
	}
	return nil
}

func deliveryJournalTerminal(j *managedDeliveryJournal) bool {
	return j.Phase == exitStepCompleted || j.Phase == exitOperationCanceled || j.Phase == exitStepFailed
}

func (s *Service) finishManagedDeliveryReceipt(ctx context.Context, journal *managedDeliveryJournal) error {
	operation := ExitOperation{ID: journal.OperationID, ProfileID: journal.Item.ProfileID,
		WorkspaceID: journal.Item.WorkspaceID, WorktreeID: journal.Item.ID, Action: string(ExitActionDeliver)}
	state, event := exitStepFailed, EventExitActionFailed
	message := "Managed delivery interrupted by daemon restart; reconciling durable intent."
	switch journal.Phase {
	case exitStepCompleted:
		state, event, message = exitStepCompleted, EventExitActionCompleted, ""
	case exitOperationCanceled:
		state, event, message = exitOperationCanceled, EventExitActionCanceled, "Managed delivery canceled."
	case exitStepFailed:
		message = journal.Failure
	}
	_, err := s.finishExitOperation(ctx, operation, state, event, ExitEventPayload{
		OperationID: operation.ID, Action: ExitActionDeliver, State: state, Result: &journal.Result, Message: message,
	})
	return err
}

func (s *Service) persistManagedDeliveryFailure(
	ctx context.Context, path string, journal *managedDeliveryJournal, cause error,
) {
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		journal.Phase = exitOperationCanceled
	case errors.Is(cause, ErrSafetyCheckFailed), errors.Is(cause, ErrExitActionInvalid):
		journal.Phase = exitStepFailed
	default:
		return // Ambiguous provider/transport failures keep their exact resumable phase.
	}
	journal.Failure = diagnostics.RedactAndBound(cause.Error(), 2048)
	if err := saveDeliveryJournal(path, journal); err != nil {
		s.logger.ErrorContext(ctx, "managed delivery failure journal failed",
			"op_id", journal.OperationID, "error", err)
	}
}
