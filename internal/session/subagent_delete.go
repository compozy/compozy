package session

import (
	"context"
	"errors"
	"fmt"

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
func (s *subagentService) Delete(
	ctx context.Context,
	actor SubagentActor,
	workspaceID, id string,
) error {
	workspace, callerSession, err := s.deleteActorScope(ctx, actor, workspaceID)
	if err != nil {
		return err
	}
	row, err := s.store.GetSubagent(ctx, workspace, id)
	if errors.Is(err, store.ErrSubagentNotFound) {
		return ErrSubagentNotFound
	}
	if err != nil {
		return fmt.Errorf("subagent delete: read %q: %w", id, err)
	}
	if callerSession != "" && row.ParentSessionID != callerSession {
		return ErrSubagentNotFound
	}
	if row.Origin == store.SubagentOriginProviderNative {
		return ErrSubagentNotDeletable
	}
	deletedChild := ""
	if row.ChildSessionID != nil {
		deletedChild = *row.ChildSessionID
		if err := s.deleteChildSession(ctx, deletedChild); err != nil {
			return fmt.Errorf("subagent delete: remove child session %q: %w", deletedChild, err)
		}
	}
	// The parent lock serializes the removal with delegation links and settlement.
	// The pre-lock read can be stale, so the current row decides: a delegation that
	// linked its child meanwhile must lose that session too, never outlive its row.
	for {
		unlock := s.lock(row.ParentSessionID)
		current, readErr := s.store.GetSubagent(ctx, workspace, id)
		if readErr != nil {
			unlock()
			if errors.Is(readErr, store.ErrSubagentNotFound) {
				return ErrSubagentNotFound
			}
			return fmt.Errorf("subagent delete: reread %q: %w", id, readErr)
		}
		child := ""
		if current.ChildSessionID != nil {
			child = *current.ChildSessionID
		}
		if child == "" || child == deletedChild {
			err = s.store.DeleteSubagent(ctx, workspace, id)
			unlock()
			break
		}
		unlock()
		if err := s.deleteChildSession(ctx, child); err != nil {
			return fmt.Errorf("subagent delete: remove child session %q: %w", child, err)
		}
		deletedChild = child
	}
	if err != nil {
		return fmt.Errorf("subagent delete: remove row %q: %w", id, err)
	}
	s.publishRemoved(row.ParentSessionID, id)
	s.runtime.PublishParent(ctx, row.ParentSessionID)
	// A completion message already composed for this row must not announce a result
	// that can no longer be read; the parent's own settlement runs on the new roster.
	s.logError(ctx, "delete_wake_rewrite", s.rewriteDeletedWake(ctx, row.ParentSessionID))
	s.logError(ctx, "delete_parent_settle", s.settleParent(ctx, row.ParentSessionID))
	s.logError(ctx, "delete_parent_propagate", s.OnChildSettled(ctx, row.ParentSessionID))
	return nil
}

// deleteActorScope resolves who owns the deletion: an operator acts on the
// requested workspace; an agent session may only remove work delegated by itself.
func (s *subagentService) deleteActorScope(
	ctx context.Context,
	actor SubagentActor,
	workspaceID string,
) (string, string, error) {
	switch actor.Kind {
	case SubagentActorOperator:
		return workspaceID, "", nil
	case string(PromptCancelAgent):
		if actor.Caller == nil || actor.Caller.WorkspaceID != workspaceID {
			return "", "", ErrSubagentNotFound
		}
		if err := s.callerSession(ctx, *actor.Caller); err != nil {
			return "", "", err
		}
		return workspaceID, actor.Caller.SessionID, nil
	default:
		return "", "", ErrSubagentNotFound
	}
}

// rewriteDeletedWake drops the deleted row from any queued completion message and
// cancels a batch that no longer carries another result.
func (s *subagentService) rewriteDeletedWake(ctx context.Context, parent string) error {
	wakes, err := s.parentWakes(ctx, parent)
	if err != nil {
		return err
	}
	for _, wake := range wakes {
		if wake.InputEntryID == "" {
			continue
		}
		_, rows, err := s.store.GetWake(ctx, wake.WakeMessageID)
		if err != nil {
			return err
		}
		if err := s.rewriteWake(ctx, wake, rows); err != nil {
			return err
		}
	}
	return nil
}

// deleteChildSession removes one child session while suppressing the settlement
// callback its deletion triggers. The parent lock is deliberately not held here:
// OnChildSettled acquires it, so holding it across the session delete would
// deadlock. A session that is already gone counts as deleted.
func (s *subagentService) deleteChildSession(ctx context.Context, child string) error {
	release := s.suppressChildSettlement(child)
	defer release()
	return s.runtime.DeleteSession(ctx, child)
}

// suppressChildSettlement keeps settlement suppressed until every overlapping
// deletion returns, not merely the first one.
func (s *subagentService) suppressChildSettlement(child string) func() {
	s.deletingChildrenMu.Lock()
	if s.deletingChildren == nil {
		s.deletingChildren = make(map[string]int)
	}
	s.deletingChildren[child]++
	s.deletingChildrenMu.Unlock()
	return func() {
		s.deletingChildrenMu.Lock()
		if count := s.deletingChildren[child]; count <= 1 {
			delete(s.deletingChildren, child)
		} else {
			s.deletingChildren[child] = count - 1
		}
		s.deletingChildrenMu.Unlock()
	}
}

// childSettlementSuppressed reports a child whose delete owns its settlement.
func (s *subagentService) childSettlementSuppressed(child string) bool {
	s.deletingChildrenMu.Lock()
	defer s.deletingChildrenMu.Unlock()
	_, suppressed := s.deletingChildren[child]
	return suppressed
}
