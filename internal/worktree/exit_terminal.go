package worktree

import (
	"context"
	"fmt"
	"strings"
)

type exitOperationControl struct {
	workspaceID string
	worktreeID  string
	cancel      context.CancelFunc
	done        chan struct{}
}

func (s *Service) finishExitOperation(
	ctx context.Context,
	operation ExitOperation,
	state string,
	eventName string,
	payload ExitEventPayload,
) (bool, error) {
	finishedAt := s.now().UTC()
	if s.events == nil || strings.TrimSpace(eventName) == "" {
		return s.store.FinishExitOperation(
			ctx, operation.WorkspaceID, operation.WorktreeID, operation.ID, state, finishedAt,
		)
	}
	event, err := exitLifecycleEvent(eventName, operation, payload)
	if err != nil {
		return false, err
	}
	if transactional, ok := s.events.(ExitTerminalEventSink); ok {
		return transactional.FinishExitOperationWithEvent(
			ctx, operation.WorkspaceID, operation.WorktreeID, operation.ID, state, finishedAt, event,
		)
	}
	if err := s.events.PublishWorktreeEvent(ctx, event); err != nil {
		return false, fmt.Errorf("worktree: publish terminal exit event: %w", err)
	}
	return s.store.FinishExitOperation(
		ctx, operation.WorkspaceID, operation.WorktreeID, operation.ID, state, finishedAt,
	)
}
