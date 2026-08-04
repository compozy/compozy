package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/task"
)

// CancelRun requests cooperative cancellation and terminalizes after prompt delivery drains.
func (s *service) CancelRun(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	reason string,
	actor task.ActorContext,
) error {
	return s.cancelRun(ctx, ws, runID, RunCancelCancel, reason, actor)
}

// KillRun commits terminal truth before stopping every bound session immediately.
func (s *service) KillRun(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	reason string,
	actor task.ActorContext,
) error {
	return s.cancelRun(ctx, ws, runID, RunCancelKill, reason, actor)
}

// CancelNode requests cooperative cancellation for one authored node across its live cells.
func (s *service) CancelNode(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	reason string,
	actor task.ActorContext,
) error {
	return s.cancelNode(ctx, ws, runID, nodeID, RunCancelCancel, reason, actor)
}

// KillNode fences one node and then immediately stops its bound sessions.
func (s *service) KillNode(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	reason string,
	actor task.ActorContext,
) error {
	return s.cancelNode(ctx, ws, runID, nodeID, RunCancelKill, reason, actor)
}

func (s *service) cancelRun(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	kind RunCancelKind,
	reason string,
	actor task.ActorContext,
) error {
	parentCloseActions, err := s.parentCloseActions(ctx, ws, runID)
	if err != nil {
		return err
	}
	store, mutation, err := s.prepareCancellation(ctx, ws, runID, "", kind, reason, actor)
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
	cause := TransitionCauseOperatorCancel
	if kind == RunCancelKill {
		cause = TransitionCauseOperatorKill
	}
	s.revokeGoalPromptLeases(ctx, result.RevokedPromptLeases, cause)
	parentCloseErr := s.applyParentCloseActions(ctx, mutation, parentCloseActions)
	if result.Terminal {
		return errors.Join(parentCloseErr, s.finishCommittedRunCancellation(ctx, mutation, &result))
	}
	if kind == RunCancelKill {
		return parentCloseErr
	}
	if _, err := store.AdvanceRunCancellation(ctx, mutation, CancelStateDelivering); err != nil {
		return errors.Join(parentCloseErr, err)
	}
	if err := s.deliverSessionCancellation(ctx, result.SessionIDs, kind, mutation.Reason); err != nil {
		return errors.Join(parentCloseErr, err)
	}
	draining, err := store.AdvanceRunCancellation(ctx, mutation, CancelStateDraining)
	if err != nil {
		return errors.Join(parentCloseErr, err)
	}
	s.activateCancellationResult(ctx, &draining)
	return parentCloseErr
}

func (s *service) cancelNode(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	kind RunCancelKind,
	reason string,
	actor task.ActorContext,
) error {
	parentCloseActions, err := s.parentCloseActions(ctx, ws, runID)
	if err != nil {
		return err
	}
	parentCloseActions = parentCloseActionsForNode(parentCloseActions, nodeID)
	store, mutation, err := s.prepareCancellation(ctx, ws, runID, nodeID, kind, reason, actor)
	if err != nil {
		return err
	}
	if kind == RunCancelCancel {
		mutation.Effects, err = s.renderNodeCanceledEffects(ctx, ws, runID, nodeID)
		if err != nil {
			return err
		}
	}
	result, err := store.RequestNodeCancellation(ctx, mutation)
	if err != nil || result.Terminal || !result.Applied {
		return err
	}
	parentCloseErr := s.applyParentCloseActions(ctx, mutation, parentCloseActions)
	if kind == RunCancelKill {
		s.activateCancellationResult(ctx, &result)
		return errors.Join(
			parentCloseErr,
			s.deliverSessionCancellation(ctx, result.SessionIDs, kind, mutation.Reason),
		)
	}
	if _, err := store.AdvanceNodeCancellation(ctx, mutation, CancelStateDelivering); err != nil {
		return errors.Join(parentCloseErr, err)
	}
	if err := s.deliverSessionCancellation(ctx, result.SessionIDs, kind, mutation.Reason); err != nil {
		return errors.Join(parentCloseErr, err)
	}
	draining, err := store.AdvanceNodeCancellation(ctx, mutation, CancelStateDraining)
	if err != nil {
		return errors.Join(parentCloseErr, err)
	}
	s.activateCancellationResult(ctx, &draining)
	return parentCloseErr
}

func (s *service) renderNodeCanceledEffects(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
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
	return RenderEffectIntents(node.OnCancel, EffectContextRequest{
		Run: &run, Trigger: EffectTriggerOnCancel, Generation: max(1, run.Generation),
		NodeID: nodeID, Attempt: 1, Disposition: AttemptCanceled,
	})
}

func (s *service) prepareCancellation(
	_ context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	kind RunCancelKind,
	reason string,
	actor task.ActorContext,
) (CancellationStore, CancellationMutation, error) {
	store, ok := s.store.(CancellationStore)
	if !ok {
		return nil, CancellationMutation{}, fmt.Errorf(
			"%w: Loop cancellation store is unavailable",
			ErrActionDependencyMissing,
		)
	}
	mutation := CancellationMutation{
		WorkspaceID: ws, RunID: runID, NodeID: nodeID, Kind: kind,
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
	kind RunCancelKind,
	reason string,
) error {
	if len(sessionIDs) == 0 {
		return nil
	}
	if s.cancellationSessions == nil {
		return fmt.Errorf("%w: Loop cancellation session controller is unavailable", ErrActionDependencyMissing)
	}
	deliveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), actionCancelMaxWait)
	defer cancel()
	var deliveryErr error
	for _, sessionID := range sessionIDs {
		var err error
		if kind == RunCancelKill {
			err = s.cancellationSessions.KillLoopSession(deliveryCtx, sessionID, reason)
		} else {
			err = s.cancellationSessions.CancelLoopSession(deliveryCtx, sessionID, reason)
		}
		if err != nil {
			deliveryErr = errors.Join(deliveryErr, fmt.Errorf("cancel Loop session %q: %w", sessionID, err))
		}
	}
	return deliveryErr
}

func (s *service) finishCommittedRunCancellation(
	ctx context.Context,
	mutation CancellationMutation,
	result *CancellationResult,
) error {
	if result == nil {
		return nil
	}
	if !result.Terminal || result.Run.Status != StatusCanceled {
		return nil
	}
	cause := TransitionCauseOperatorCancel
	if mutation.Kind == RunCancelKill {
		cause = TransitionCauseOperatorKill
	}
	s.dispatchCoordinatorTerminal(ctx, result.Run, cause, mutation.RequestedAt)
	if mutation.Kind != RunCancelKill {
		return nil
	}
	return s.deliverSessionCancellation(ctx, result.SessionIDs, mutation.Kind, mutation.Reason)
}

func (s *service) activateCancellationResult(ctx context.Context, result *CancellationResult) {
	if result == nil || s.coordinatorActivator == nil || result.Coordinator == nil {
		return
	}
	s.coordinatorActivator.ActivateCoordinatorRun(context.WithoutCancel(ctx), *result.Coordinator)
}
