package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/task"
)

// RequeueNode clears one quarantine episode and reserves the next coordinator generation.
func (s *service) RequeueNode(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	reason string,
	actor task.ActorContext,
) (NodeRequeueResult, error) {
	store, ok := s.store.(NodeRequeueStore)
	if !ok {
		return NodeRequeueResult{}, fmt.Errorf("%w: node requeue store is unavailable", ErrActionDependencyMissing)
	}
	result, err := store.RequeueNode(ctx, NodeRequeueMutation{
		WorkspaceID: ws,
		RunID:       runID,
		NodeID:      nodeID,
		Reason:      strings.TrimSpace(reason),
		Actor:       actor,
		RequestedAt: s.now().UTC(),
	})
	if err != nil {
		return NodeRequeueResult{}, err
	}
	if s.coordinatorActivator != nil {
		s.coordinatorActivator.ActivateCoordinatorRun(context.WithoutCancel(ctx), result.Coordinator)
	}
	return result, nil
}

// ResumeNodeWait atomically claims one manual wait payload and wakes its coordinator.
func (s *service) ResumeNodeWait(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	nodeID NodeID,
	itemIndex int,
	payload json.RawMessage,
	actor task.ActorContext,
) (WaitResumeResult, error) {
	store, ok := s.store.(WaitAdmission)
	if !ok {
		return WaitResumeResult{}, fmt.Errorf("%w: wait admission store is unavailable", ErrActionDependencyMissing)
	}
	run, err := s.Get(ctx, ws, runID)
	if err != nil {
		return WaitResumeResult{}, err
	}
	resolved, err := s.pinnedResolvedDefinition(ctx, *run)
	if err != nil {
		return WaitResumeResult{}, err
	}
	claimedByKind := string(actor.Actor.Kind.Normalize())
	claimedByID := strings.TrimSpace(actor.Actor.Ref)
	if claimedByID == "" {
		claimedByID = claimedByKind
	}
	defaults := DefaultLifecycleConfig()
	admissionAttempts := *defaults.WaitAdmissionAttempts
	if resolved.EffectiveConfig.Lifecycle.WaitAdmissionAttempts != nil {
		admissionAttempts = *resolved.EffectiveConfig.Lifecycle.WaitAdmissionAttempts
	}
	result, err := store.ResumeWait(ctx, WaitResumeMutation{
		WorkspaceID:       ws,
		RunID:             runID,
		Generation:        run.Generation,
		NodeID:            nodeID,
		ItemIndex:         itemIndex,
		Payload:           append(json.RawMessage(nil), payload...),
		ClaimedByKind:     claimedByKind,
		ClaimedByID:       claimedByID,
		AdmissionAttempts: admissionAttempts,
		RequestedAt:       s.now().UTC(),
	})
	if err != nil {
		return WaitResumeResult{}, err
	}
	if result.Coordinator != nil && s.coordinatorActivator != nil {
		s.coordinatorActivator.ActivateCoordinatorRun(context.WithoutCancel(ctx), *result.Coordinator)
	}
	return result, nil
}

// NodeLifecycleService exposes node-scoped control verbs implemented by the aggregate.
type NodeLifecycleService interface {
	PauseNode(
		context.Context,
		WorkspaceID,
		RunID,
		NodeID,
		NodePauseMode,
		string,
		task.ActorContext,
	) (NodePauseResult, error)
	ResumeNode(
		context.Context,
		WorkspaceID,
		RunID,
		NodeID,
		NodeResumeMode,
		task.ActorContext,
	) (NodeResumeResult, error)
	ResumeNodeWait(
		context.Context,
		WorkspaceID,
		RunID,
		NodeID,
		int,
		json.RawMessage,
		task.ActorContext,
	) (WaitResumeResult, error)
	RequeueNode(
		context.Context,
		WorkspaceID,
		RunID,
		NodeID,
		string,
		task.ActorContext,
	) (NodeRequeueResult, error)
}

var _ NodeLifecycleService = (*service)(nil)
