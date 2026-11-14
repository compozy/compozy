package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

// Delete permanently removes a delegated subagent together with its child
// session, whose transcript and history go away exactly as they do for a
// session deletion. Provider-native rows describe a tool call inside the
// parent's turn: they follow that turn and cannot be removed here.
//
// Ordering: the child session is deleted first and the roster row second, so a
// failed session deletion leaves the subagent visible and retryable instead of
// stranding an unreachable child. The child's delete settles it through the
// manager, which calls back into OnChildSettled; that callback is suppressed
// for this deletion so the row is not finalized or hooked on its way out.
func (s *subagentService) Delete(ctx context.Context, workspaceID, id string) error {
	row, err := s.store.GetSubagent(ctx, workspaceID, id)
	if errors.Is(err, store.ErrSubagentNotFound) {
		return ErrSubagentNotFound
	}
	if err != nil {
		return err
	}
	if row.Origin == store.SubagentOriginProviderNative {
		return ErrSubagentNotDeletable
	}
	deletedChild := ""
	if row.ChildSessionID != nil {
		deletedChild = *row.ChildSessionID
		if err := s.deleteChildSession(ctx, deletedChild); err != nil {
			return err
		}
	}
	// The parent lock serializes the removal with delegation links and settlement.
	// The pre-lock read can be stale, so the current row decides: a delegation that
	// linked its child meanwhile must lose that session too, never outlive its row.
	for {
		unlock := s.lock(row.ParentSessionID)
		current, readErr := s.store.GetSubagent(ctx, workspaceID, id)
		if readErr != nil {
			unlock()
			if errors.Is(readErr, store.ErrSubagentNotFound) {
				return ErrSubagentNotFound
			}
			return readErr
		}
		child := ""
		if current.ChildSessionID != nil {
			child = *current.ChildSessionID
		}
		if child == "" || child == deletedChild {
			err = s.store.DeleteSubagent(ctx, workspaceID, id)
			unlock()
			break
		}
		unlock()
		if err := s.deleteChildSession(ctx, child); err != nil {
			return err
		}
		deletedChild = child
	}
	if err != nil {
		return err
	}
	s.publishRemoved(row.ParentSessionID, id)
	s.runtime.PublishParent(ctx, row.ParentSessionID)
	return nil
}

// deleteChildSession removes one child session while suppressing the settlement
// callback its deletion triggers. The parent lock is deliberately not held here:
// OnChildSettled acquires it, so holding it across the session delete would
// deadlock. A session that is already gone counts as deleted.
func (s *subagentService) deleteChildSession(ctx context.Context, child string) (err error) {
	s.deletingChildren.Store(child, struct{}{})
	defer s.deletingChildren.Delete(child)
	return s.runtime.DeleteSession(ctx, child)
}

// childSettlementSuppressed reports a child whose delete owns its settlement.
func (s *subagentService) childSettlementSuppressed(child string) bool {
	_, suppressed := s.deletingChildren.Load(child)
	return suppressed
}
