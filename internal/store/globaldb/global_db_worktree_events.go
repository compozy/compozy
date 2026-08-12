package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/worktree"
)

func (g *GlobalDB) PublishWorktreeEvent(ctx context.Context, event worktree.LifecycleEvent) error {
	if g == nil || g.ObserveRepo == nil {
		return fmt.Errorf("store: worktree event writer is required")
	}
	return g.ObserveRepo.WriteEventSummary(ctx, worktreeEventSummary(event))
}

func (g *GlobalDB) FinishExitOperationWithEvent(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
	opID string,
	state string,
	finishedAt time.Time,
	event worktree.LifecycleEvent,
) (bool, error) {
	if g == nil || g.Worktrees == nil || g.ObserveRepo == nil {
		return false, fmt.Errorf("store: worktree terminal event writer is required")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	worktreeID = strings.TrimSpace(worktreeID)
	if workspaceID != strings.TrimSpace(event.WorkspaceID) || worktreeID != strings.TrimSpace(event.WorktreeID) {
		return false, fmt.Errorf("store: worktree terminal event scope does not match operation")
	}
	summary := worktreeEventSummary(event)
	if err := g.ObserveRepo.prepareEventSummary(ctx, &summary); err != nil {
		return false, err
	}
	finished := false
	err := g.Worktrees.withImmediateTransaction(ctx, "finish worktree exit operation with event", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		count, err := queries.FinishWorktreeExitOperation(ctx, sqlcgen.FinishWorktreeExitOperationParams{
			State: state, FinishedAt: nullableTime(&finishedAt), WorkspaceID: workspaceID,
			WorktreeID: worktreeID, OpID: strings.TrimSpace(opID),
		})
		if err != nil {
			return fmt.Errorf("finish worktree exit operation: %w", err)
		}
		if count != 1 {
			return nil
		}
		if err := insertEventSummary(ctx, queries, summary); err != nil {
			return err
		}
		finished = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return finished, nil
}

func worktreeEventSummary(event worktree.LifecycleEvent) store.EventSummary {
	outcome := string(eventspkg.OutcomeFor(event.Name))
	content := append(json.RawMessage(nil), event.Payload...)
	return store.EventSummary{
		WorkspaceID: event.WorkspaceID,
		Type:        event.Name,
		Outcome:     outcome,
		Content:     content,
		EventCorrelation: store.EventCorrelation{
			WorktreeID: event.WorktreeID,
			RunID:      event.RunID,
		},
	}
}

var _ worktree.EventSink = (*GlobalDB)(nil)
var _ worktree.ExitTerminalEventSink = (*GlobalDB)(nil)
