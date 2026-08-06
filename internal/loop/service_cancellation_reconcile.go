package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/task"
)

var _ CancellationReconciler = (*service)(nil)

// ReconcilePendingCancellations advances a bounded set of committed cooperative cancellations.
func (s *service) ReconcilePendingCancellations(
	ctx context.Context,
	limit int,
	actor task.ActorContext,
) (int, error) {
	if ctx == nil {
		return 0, fmt.Errorf("%w: cancellation reconciliation context is required", ErrValidation)
	}
	if limit <= 0 {
		return 0, fmt.Errorf("%w: cancellation reconciliation limit must be positive", ErrValidation)
	}
	if err := actor.Validate(); err != nil {
		return 0, fmt.Errorf("%w: cancellation reconciliation actor: %w", ErrValidation, err)
	}
	store, ok := s.store.(CancellationReconciliationStore)
	if !ok {
		return 0, fmt.Errorf("%w: Loop cancellation reconciliation store is unavailable", ErrActionDependencyMissing)
	}
	pending, err := store.ListPendingCancellations(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("list pending Loop cancellations: %w", err)
	}

	reconciled := 0
	var reconcileErr error
	for _, command := range pending {
		if err := s.reconcilePendingCancellation(ctx, store, command, actor); err != nil {
			reconcileErr = errors.Join(reconcileErr, fmt.Errorf(
				"reconcile Loop cancellation %q/%q: %w",
				command.WorkspaceID,
				command.RunID,
				err,
			))
			continue
		}
		reconciled++
	}
	return reconciled, reconcileErr
}

func (s *service) reconcilePendingCancellation(
	ctx context.Context,
	store CancellationStore,
	command PendingCancellation,
	actor task.ActorContext,
) error {
	mutation := CancellationMutation{
		WorkspaceID: command.WorkspaceID,
		RunID:       command.RunID,
		NodeID:      command.NodeID,
		Kind:        RunCancelCancel,
		Reason:      strings.TrimSpace(command.Reason),
		Actor:       actor,
		RequestedAt: command.RequestedAt,
	}
	if err := mutation.Validate(command.NodeID != ""); err != nil {
		return err
	}

	if command.State == CancelStateRequested {
		result, err := advanceCancellation(ctx, store, mutation, CancelStateDelivering)
		if err != nil {
			return err
		}
		if result.Terminal {
			return nil
		}
	} else if command.State != CancelStateDelivering {
		return fmt.Errorf("%w: cancellation state %q is not reconcilable", ErrValidation, command.State)
	}

	if err := s.deliverSessionCancellation(ctx, command.SessionIDs, mutation.Kind, mutation.Reason); err != nil {
		return err
	}
	draining, err := advanceCancellation(ctx, store, mutation, CancelStateDraining)
	if err != nil {
		return err
	}
	s.activateCancellationResult(ctx, &draining)
	return nil
}

func advanceCancellation(
	ctx context.Context,
	store CancellationStore,
	mutation CancellationMutation,
	state CancelState,
) (CancellationResult, error) {
	if mutation.NodeID == "" {
		return store.AdvanceRunCancellation(ctx, mutation, state)
	}
	return store.AdvanceNodeCancellation(ctx, mutation, state)
}

func (s *service) deferCancellationDelivery(
	ctx context.Context,
	store CancellationStore,
	mutation CancellationMutation,
	result *CancellationResult,
) {
	command := PendingCancellation{
		WorkspaceID: mutation.WorkspaceID,
		RunID:       mutation.RunID,
		NodeID:      mutation.NodeID,
		State:       CancelStateRequested,
		Reason:      mutation.Reason,
		RequestedAt: mutation.RequestedAt,
		RequestedBy: mutation.Actor.Actor,
		SessionIDs:  append([]string(nil), result.SessionIDs...),
	}
	if err := s.reconcilePendingCancellation(ctx, store, command, mutation.Actor); err != nil {
		s.logger.Warn(
			"loop cancellation delivery deferred",
			"workspace_id", mutation.WorkspaceID,
			"run_id", mutation.RunID,
			"node_id", mutation.NodeID,
			"actor_kind", mutation.Actor.Actor.Kind,
			"actor_id", mutation.Actor.Actor.Ref,
			"error", err,
		)
	}
}
