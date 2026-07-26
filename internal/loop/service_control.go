package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
)

func (s *service) Stop(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	reason StopReason,
	actor task.ActorContext,
) error {
	if err := actor.Validate(); err != nil {
		return fmt.Errorf("%w: actor context: %w", ErrValidation, err)
	}
	if reason == "" {
		reason = StopReasonOperator
	}
	if reason != StopReasonOperator {
		return fmt.Errorf("%w: stop reason is invalid: %q", ErrValidation, reason)
	}
	run, err := s.store.GetLoopRun(ctx, ws, runID)
	if err != nil {
		return err
	}
	if run.Status.Terminal() {
		return reasonError(
			ReasonCodeTerminalRun,
			ErrInvalidTransition,
			map[string]string{reasonMetaRunID: string(runID), reasonMetaStatus: string(run.Status)},
		)
	}
	if !allowedTransition(run.Status, StatusFailed, TransitionCauseOperatorStop) {
		return reasonError(
			ReasonCodeInvalidStatusTransition,
			ErrInvalidTransition,
			map[string]string{
				reasonMetaCause: string(TransitionCauseOperatorStop),
				reasonMetaFrom:  string(run.Status),
				reasonMetaTo:    string(StatusFailed),
			},
		)
	}
	if stopper, ok := s.store.(GoalRunStopStore); ok {
		stoppedAt := s.now().UTC()
		result, err := stopper.StopGoalRun(ctx, GoalRunStopRequest{
			WorkspaceID:    ws,
			RunID:          runID,
			ExpectedStatus: run.Status,
			Actor:          actor,
			StoppedAt:      stoppedAt,
		})
		if err != nil {
			return err
		}
		s.revokeGoalPromptLeases(ctx, result.RevokedPromptLeases, TransitionCauseOperatorStop)
		run.Status = StatusFailed
		s.dispatchCoordinatorTerminal(ctx, run, TransitionCauseOperatorStop, stoppedAt)
		return nil
	}
	return s.Transition(ctx, runID, StatusFailed, TransitionCauseOperatorStop)
}

func (s *service) Pause(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	actor task.ActorContext,
) error {
	if err := actor.Validate(); err != nil {
		return fmt.Errorf("%w: actor context: %w", ErrValidation, err)
	}
	run, err := s.store.GetLoopRun(ctx, ws, runID)
	if err != nil {
		return err
	}
	if run.Status != StatusRunning {
		return reasonError(
			ReasonCodeInvalidStatusTransition,
			ErrInvalidTransition,
			map[string]string{reasonMetaFrom: string(run.Status), reasonMetaTo: string(StatusPaused)},
		)
	}
	if run.PauseRequested {
		return nil
	}
	return s.store.SetLoopRunPauseRequested(ctx, ws, runID, true, actor)
}

func (s *service) Resume(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	actor task.ActorContext,
) error {
	if err := actor.Validate(); err != nil {
		return fmt.Errorf("%w: actor context: %w", ErrValidation, err)
	}
	run, err := s.store.GetLoopRun(ctx, ws, runID)
	if err != nil {
		return err
	}
	switch run.Status {
	case StatusRunning:
		if !run.PauseRequested {
			return nil
		}
		return s.store.SetLoopRunPauseRequested(ctx, ws, runID, false, actor)
	case StatusPaused:
		if handled, err := s.reactivateGoalControl(ctx, run, GateDecisionApprove, actor); handled || err != nil {
			return err
		}
		if err := s.Transition(ctx, runID, StatusRunning, TransitionCauseOperatorResume); err != nil {
			return err
		}
		return s.store.SetLoopRunPauseRequested(ctx, ws, runID, false, actor)
	default:
		return reasonError(
			ReasonCodeInvalidStatusTransition,
			ErrInvalidTransition,
			map[string]string{reasonMetaFrom: string(run.Status), reasonMetaTo: string(StatusRunning)},
		)
	}
}

func (s *service) Approve(
	ctx context.Context,
	ws WorkspaceID,
	runID RunID,
	gateID NodeID,
	decision GateDecision,
	actor task.ActorContext,
) error {
	if gateID == "" {
		return fmt.Errorf("%w: gate id is required", ErrValidation)
	}
	if err := actor.Validate(); err != nil {
		return fmt.Errorf("%w: actor context: %w", ErrValidation, err)
	}
	run, err := s.store.GetLoopRun(ctx, ws, runID)
	if err != nil {
		return err
	}
	if run.Status != StatusNeedsApproval {
		return reasonError(
			ReasonCodeInvalidStatusTransition,
			ErrInvalidTransition,
			map[string]string{reasonMetaFrom: string(run.Status), reasonMetaTo: string(StatusRunning)},
		)
	}
	if strings.TrimSpace(string(run.ActiveGateID)) != strings.TrimSpace(string(gateID)) {
		return reasonError(
			ReasonCodeInvalidStatusTransition,
			ErrInvalidTransition,
			map[string]string{
				reasonMetaRunID:  string(runID),
				"active_gate_id": strings.TrimSpace(string(run.ActiveGateID)),
				"gate_id":        strings.TrimSpace(string(gateID)),
			},
		)
	}
	if decision == GateDecisionApprove {
		if handled, err := s.reactivateGoalControl(ctx, run, decision, actor); handled || err != nil {
			return err
		}
	}
	switch decision {
	case GateDecisionApprove, GateDecisionRequestChanges:
		if gateID == BudgetGateID && decision == GateDecisionRequestChanges {
			return fmt.Errorf("%w: budget gate only accepts approve or reject", ErrValidation)
		}
		if err := s.recordGateDecision(ctx, run, gateID, decision, actor); err != nil {
			return err
		}
		return s.Transition(ctx, runID, StatusRunning, TransitionCauseApproval)
	case GateDecisionReject:
		if err := s.recordGateDecision(ctx, run, gateID, decision, actor); err != nil {
			return err
		}
		return s.Transition(ctx, runID, StatusBlocked, TransitionCauseGateRejected)
	default:
		return fmt.Errorf("%w: gate decision is invalid: %q", ErrValidation, decision)
	}
}

func (s *service) reactivateGoalControl(
	ctx context.Context,
	run Run,
	decision GateDecision,
	actor task.ActorContext,
) (bool, error) {
	controls, ok := s.store.(GoalControlStore)
	if !ok {
		return false, nil
	}
	state, found, err := controls.LoadAwaitingGoalControl(ctx, run.WorkspaceID, run.ID)
	if err != nil || !found {
		return found, err
	}
	if state.RunStatus != run.Status || state.GateID != run.ActiveGateID {
		return true, reasonError(
			ReasonCodeGoalControlStale,
			ErrTransitionConflict,
			map[string]string{reasonMetaRunID: string(run.ID)},
		)
	}
	kind, scope, increment, err := s.goalGrantForControl(ctx, run, state)
	if err != nil {
		return true, err
	}
	var decisions []GateDecisionRecord
	if run.Status == StatusNeedsApproval {
		decisions, err = gateDecisionRecords(run, run.ActiveGateID, decision, actor, s.now().UTC())
		if err != nil {
			return true, err
		}
	}
	result, err := controls.ReactivateGoalRun(ctx, GoalReactivationRequest{
		State:         state,
		Kind:          kind,
		Scope:         scope,
		TurnIncrement: increment,
		Actor:         actor,
		Decisions:     decisions,
	})
	if err != nil {
		return true, err
	}
	if s.goalRunActivator != nil {
		s.goalRunActivator.ActivateGoalRun(context.WithoutCancel(ctx), result.Run)
	}
	return true, nil
}

func (s *service) goalGrantForControl(
	ctx context.Context,
	run Run,
	state GoalControlState,
) (GoalGrantKind, GoalGrantScope, int, error) {
	switch state.Cause {
	case ReasonCodeGoalTurnsExhausted:
		increment, err := s.pinnedGoalTurnIncrement(ctx, run, state.NodeID)
		return GoalGrantTurnExtension, GoalGrantScopeTurnLimit, increment, err
	case ReasonCodeGoalReseedConfirmationRequired:
		return GoalGrantReseed, GoalGrantScopeRotateBinding, 0, nil
	case ReasonCodeGoalBudgetFenced:
		scope := GoalGrantScopeWorkAndSettle
		if state.OpenTurn {
			scope = GoalGrantScopeSettleCurrent
		}
		return GoalGrantBudget, scope, 0, nil
	default:
		return GoalGrantPlainResume, GoalGrantScopeReactivate, 0, nil
	}
}

func (s *service) pinnedGoalTurnIncrement(
	ctx context.Context,
	run Run,
	nodeID dsl.NodeID,
) (int, error) {
	snapshot, err := s.store.GetLoopDefinitionSnapshot(ctx, run.WorkspaceID, run.DefinitionDigest)
	if err != nil {
		return 0, err
	}
	resolved, err := LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
	if err != nil {
		return 0, err
	}
	node, ok := findGoalNode(resolved.Definition.Graph, nodeID)
	if !ok {
		return 0, fmt.Errorf("%w: pinned Goal node %q not found", ErrValidation, nodeID)
	}
	var params dsl.GoalParams
	if err := node.Params.Decode(&params); err != nil {
		return 0, fmt.Errorf("%w: decode pinned Goal node %q: %w", ErrValidation, nodeID, err)
	}
	if params.MaxTurns < 1 {
		return 0, fmt.Errorf("%w: pinned Goal max_turns must be positive", ErrValidation)
	}
	return params.MaxTurns, nil
}

func findGoalNode(graph dsl.Graph, nodeID dsl.NodeID) (dsl.Node, bool) {
	for _, node := range graph.Nodes {
		if node.ID == nodeID && dsl.ActionKind(node.Kind) == dsl.ActionGoal {
			return node, true
		}
		if node.Body != nil {
			if nested, ok := findGoalNode(*node.Body, nodeID); ok {
				return nested, true
			}
		}
	}
	return dsl.Node{}, false
}

func (s *service) recordGateDecision(
	ctx context.Context,
	run Run,
	gateID NodeID,
	decision GateDecision,
	actor task.ActorContext,
) error {
	records, err := gateDecisionRecords(run, gateID, decision, actor, s.now().UTC())
	if err != nil {
		return err
	}
	return s.store.RecordLoopGateDecisions(ctx, records)
}
