package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/task"
)

// PauseNode parks one authored node across its live cells.
func (s *service) PauseNode(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	mode NodePauseMode,
	reason string,
	actor task.ActorContext,
) (NodePauseResult, error) {
	store, ok := s.store.(NodePauseStore)
	if !ok {
		return NodePauseResult{}, fmt.Errorf("%w: node pause store is unavailable", ErrActionDependencyMissing)
	}
	mutation := NodePauseMutation{
		WorkspaceID: ws,
		RunID:       runID,
		NodeID:      nodeID,
		Mode:        mode,
		Reason:      strings.TrimSpace(reason),
		Actor:       actor,
		RequestedAt: s.now().UTC(),
	}
	result, err := store.PauseNode(ctx, mutation)
	if err != nil || mode != NodePauseCancel || len(result.SessionIDs) == 0 {
		return result, err
	}
	if err := s.deliverSessionCancellation(ctx, result.SessionIDs, RunCancelCancel, mutation.Reason); err != nil {
		return result, err
	}
	return result, nil
}

// ResumeNode releases one node pause using the requested retry policy.
func (s *service) ResumeNode(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	mode NodeResumeMode,
	actor task.ActorContext,
) (NodeResumeResult, error) {
	store, ok := s.store.(NodePauseStore)
	if !ok {
		return NodeResumeResult{}, fmt.Errorf("%w: node pause store is unavailable", ErrActionDependencyMissing)
	}
	result, err := store.ResumeNode(ctx, NodeResumeMutation{
		WorkspaceID: ws,
		RunID:       runID,
		NodeID:      nodeID,
		Mode:        mode,
		Actor:       actor,
		RequestedAt: s.now().UTC(),
	})
	if err != nil {
		return NodeResumeResult{}, err
	}
	if result.Coordinator != nil && s.coordinatorActivator != nil {
		s.coordinatorActivator.ActivateCoordinatorRun(context.WithoutCancel(ctx), *result.Coordinator)
	}
	return result, nil
}
