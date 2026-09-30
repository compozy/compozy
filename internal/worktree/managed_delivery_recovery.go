package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/diagnostics"
)

var ErrDeliveryInventoryUnavailable = errors.New("managed delivery journal inventory unavailable")

// RecoverManagedDeliveries repairs receipts without replaying unknown or terminal intents.
func (s *Service) RecoverManagedDeliveries(ctx context.Context) error {
	journals, complete := s.readManagedDeliveryJournals(ctx)
	if !complete {
		return ErrDeliveryInventoryUnavailable
	}
	if err := s.failUnjournaledDeliveries(ctx, journals); err != nil {
		return err
	}
	for _, journal := range journals {
		active, err := s.repairManagedDeliveryReceipt(ctx, journal)
		if err != nil {
			return err
		}
		if active || deliveryJournalTerminal(journal) || s.deliverySessions == nil {
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

// Admission and receipt repair share the control lock so a live operation cannot look orphaned.
func (s *Service) failUnjournaledDeliveries(ctx context.Context, journals []*managedDeliveryJournal) error {
	s.exitMu.Lock()
	defer s.exitMu.Unlock()
	running, err := s.store.ListRunningExitOperations(ctx)
	if err != nil {
		return err
	}
	for _, operation := range running {
		if ExitAction(operation.Action) != ExitActionDeliver || s.exits[operation.ID] != nil {
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
	return nil
}

func (s *Service) repairManagedDeliveryReceipt(ctx context.Context, journal *managedDeliveryJournal) (bool, error) {
	s.exitMu.Lock()
	defer s.exitMu.Unlock()
	if s.exits[journal.OperationID] != nil {
		return true, nil
	}
	return false, s.finishManagedDeliveryReceipt(ctx, journal)
}

// Failed inventory cannot establish that an operation has no journal.
func (s *Service) readManagedDeliveryJournals(ctx context.Context) ([]*managedDeliveryJournal, bool) {
	if s.root == "" {
		return nil, true
	}
	entries, err := os.ReadDir(filepath.Join(s.root, ".delivery"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, true
	}
	if err != nil {
		s.logger.ErrorContext(ctx, "managed delivery journals unavailable", "error", err)
		return nil, false
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
	return journals, true
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
