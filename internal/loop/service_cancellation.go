package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/task"
	"golang.org/x/sync/errgroup"
)

// CancelRun commits terminal truth before stopping every owned session.
func (s *service) CancelRun(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	reason string,
	actor task.ActorContext,
) error {
	return s.cancelRun(ctx, ws, runID, reason, actor)
}

// CancelNode fences and terminalizes one authored node across its live cells.
func (s *service) CancelNode(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	itemIndex *int,
	reason string,
	actor task.ActorContext,
) error {
	return s.cancelNode(ctx, ws, runID, nodeID, itemIndex, reason, actor)
}

func (s *service) cancelRun(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	reason string,
	actor task.ActorContext,
) error {
	parentCloseActions, err := s.parentCloseActions(ctx, ws, runID)
	if err != nil {
		return err
	}
	store, mutation, err := s.prepareCancellation(ctx, ws, runID, "", reason, actor)
	if err != nil {
		return err
	}
	mutation.Effects, err = s.renderCanceledEffects(ctx, ws, runID)
	if err != nil {
		return err
	}
	result, err := store.RequestRunCancellation(ctx, mutation)
	if err != nil {
		return err
	}
	if !result.Applied {
		return nil
	}
	s.revokeGoalPromptLeases(ctx, result.RevokedPromptLeases, TransitionCauseOperatorCancel)
	parentCloseErr := s.applyParentCloseActions(ctx, mutation, parentCloseActions)
	if result.Terminal {
		s.finishCommittedRunCancellation(ctx, mutation, &result)
	}
	return parentCloseErr
}

func (s *service) cancelNode(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	itemIndex *int,
	reason string,
	actor task.ActorContext,
) error {
	parentCloseActions, err := s.parentCloseActions(ctx, ws, runID)
	if err != nil {
		return err
	}
	parentCloseActions = parentCloseActionsForNode(parentCloseActions, nodeID, itemIndex)
	store, mutation, err := s.prepareCancellation(ctx, ws, runID, nodeID, reason, actor)
	if err != nil {
		return err
	}
	mutation.ItemIndex = cloneIntPointer(itemIndex)
	mutation.Effects, err = s.renderNodeCanceledEffects(ctx, ws, runID, nodeID, itemIndex)
	if err != nil {
		return err
	}
	result, err := store.RequestNodeCancellation(ctx, mutation)
	if err != nil || result.Terminal || !result.Applied {
		return err
	}
	parentCloseErr := s.applyParentCloseActions(ctx, mutation, parentCloseActions)
	s.activateCancellationResult(ctx, &result)
	s.stopCancellationSessions(ctx, mutation, result.SessionIDs)
	return parentCloseErr
}

func (s *service) renderNodeCanceledEffects(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	itemIndex *int,
) ([]RenderedEffectIntent, error) {
	run, err := s.store.GetLoopRun(ctx, ws, runID)
	if err != nil || run.Status.Terminal() {
		return nil, err
	}
	if strings.TrimSpace(run.DefinitionDigest) == "" {
		return nil, nil
	}
	snapshot, err := s.store.GetLoopDefinitionSnapshot(ctx, ws, run.DefinitionDigest)
	if err != nil {
		return nil, err
	}
	resolved, err := LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
	if err != nil {
		return nil, err
	}
	node, found := graphNode(resolved.Definition.Graph, nodeID)
	if !found {
		return nil, fmt.Errorf("%w: Loop node %q is not present in the executed definition", ErrValidation, nodeID)
	}
	resolvedItemIndex := 0
	if itemIndex != nil {
		resolvedItemIndex = *itemIndex
	}
	return RenderEffectIntents(node.OnCancel, EffectContextRequest{
		Run: &run, Trigger: EffectTriggerOnCancel, Generation: max(1, run.Generation),
		NodeID: nodeID, ItemIndex: resolvedItemIndex, Attempt: 1, Disposition: AttemptCanceled,
	})
}

func (s *service) prepareCancellation(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	reason string,
	actor task.ActorContext,
) (CancellationStore, CancellationMutation, error) {
	if err := s.requireMutableRun(ctx, ws, runID); err != nil {
		return nil, CancellationMutation{}, err
	}
	store, ok := s.store.(CancellationStore)
	if !ok {
		return nil, CancellationMutation{}, fmt.Errorf(
			"%w: Loop cancellation store is unavailable",
			ErrActionDependencyMissing,
		)
	}
	mutation := CancellationMutation{
		WorkspaceID: ws, RunID: runID, NodeID: nodeID,
		Reason: strings.TrimSpace(reason), Actor: actor, RequestedAt: s.now().UTC(),
	}
	if err := mutation.Validate(nodeID != ""); err != nil {
		return nil, CancellationMutation{}, err
	}
	return store, mutation, nil
}

func (s *service) renderCanceledEffects(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
) ([]RenderedEffectIntent, error) {
	run, err := s.store.GetLoopRun(ctx, ws, runID)
	if err != nil || run.Status.Terminal() {
		return nil, err
	}
	if strings.TrimSpace(run.DefinitionDigest) == "" {
		return nil, nil
	}
	snapshot, err := s.store.GetLoopDefinitionSnapshot(ctx, ws, run.DefinitionDigest)
	if err != nil {
		return nil, err
	}
	resolved, err := LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
	if err != nil {
		return nil, err
	}
	effects, trigger := terminalEffectSpecs(resolved.Definition.Contract, StatusCanceled)
	return RenderEffectIntents(effects, EffectContextRequest{
		Run: &run, Trigger: trigger, Generation: max(1, run.Generation),
	})
}

func (s *service) deliverSessionCancellation(
	ctx context.Context,
	sessionIDs []string,
	reason string,
) error {
	if len(sessionIDs) == 0 {
		return nil
	}
	if s.cancellationSessions == nil {
		return fmt.Errorf("%w: Loop cancellation session controller is unavailable", ErrActionDependencyMissing)
	}
	deliveryCtx := context.WithoutCancel(ctx)
	var group errgroup.Group
	group.SetLimit(s.cancellationLimit)
	for _, sessionID := range sessionIDs {
		group.Go(func() error {
			stopCtx, cancel := context.WithTimeout(deliveryCtx, actionCancelMaxWait)
			defer cancel()
			if err := s.cancellationSessions.StopLoopSession(stopCtx, sessionID, reason); err != nil {
				return fmt.Errorf("stop Loop session %q: %w", sessionID, err)
			}
			return nil
		})
	}
	return group.Wait()
}

func (s *service) finishCommittedRunCancellation(
	ctx context.Context,
	mutation CancellationMutation,
	result *CancellationResult,
) {
	if result == nil {
		return
	}
	if !result.Terminal || result.Run.Status != StatusCanceled {
		return
	}
	s.dispatchCoordinatorTerminal(ctx, result.Run, TransitionCauseOperatorCancel, mutation.RequestedAt)
	s.stopCancellationSessions(ctx, mutation, result.SessionIDs)
}

func (s *service) stopCancellationSessions(
	ctx context.Context,
	mutation CancellationMutation,
	sessionIDs []string,
) {
	if err := s.deliverSessionCancellation(ctx, sessionIDs, mutation.Reason); err != nil {
		s.logger.WarnContext(
			ctx,
			"loop cancellation session stop deferred",
			"workspace_id", mutation.WorkspaceID,
			"run_id", mutation.RunID,
			"node_id", mutation.NodeID,
			"actor_kind", mutation.Actor.Actor.Kind,
			"actor_id", mutation.Actor.Actor.Ref,
			"error", err,
		)
	}
}

func (s *service) activateCancellationResult(ctx context.Context, result *CancellationResult) {
	if result == nil || s.coordinatorActivator == nil || result.Coordinator == nil {
		return
	}
	s.coordinatorActivator.ActivateCoordinatorRun(context.WithoutCancel(ctx), *result.Coordinator)
}
