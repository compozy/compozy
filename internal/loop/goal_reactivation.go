package loop

import (
	"context"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
)

// GoalGrantKind classifies one checkpoint-local reactivation permit.
type GoalGrantKind string

const (
	GoalGrantTurnExtension GoalGrantKind = "turn-extension"
	GoalGrantBudget        GoalGrantKind = "budget"
	GoalGrantReseed        GoalGrantKind = "reseed"
	GoalGrantPlainResume   GoalGrantKind = "plain-resume"
)

// GoalGrantScope constrains the exact operation authorized by a Goal grant.
type GoalGrantScope string

const (
	GoalGrantScopeTurnLimit     GoalGrantScope = "turn-limit"
	GoalGrantScopeSettleCurrent GoalGrantScope = "settle-current"
	GoalGrantScopeWorkAndSettle GoalGrantScope = "work-and-settle"
	GoalGrantScopeRotateBinding GoalGrantScope = "rotate-binding"
	GoalGrantScopeReactivate    GoalGrantScope = "reactivate"
)

// GoalControlState is the neutral parent-owned view of one awaiting Goal checkpoint.
type GoalControlState struct {
	WorkspaceID  WorkspaceID
	LoopRunID    RunID
	Generation   int
	NodeID       dsl.NodeID
	ItemIndex    int
	ControlEpoch int64
	GoalStatus   string
	TurnsUsed    int
	TurnLimit    int
	TaskRunID    string
	QueueEntryID string
	PromptID     string
	PromptKind   string
	OpenTurn     bool
	GateID       NodeID
	Cause        ReasonCode
	RunStatus    Status
}

// GoalReactivationRequest atomically grants and enqueues one successor Goal segment.
type GoalReactivationRequest struct {
	State         GoalControlState
	Kind          GoalGrantKind
	Scope         GoalGrantScope
	TurnIncrement int
	Actor         task.ActorContext
	Decisions     []GateDecisionRecord
}

// GoalReactivationResult records the monotonic grant and exactly-one successor identity.
type GoalReactivationResult struct {
	Run          task.Run
	ControlEpoch int64
	GrantID      int64
	Existing     bool
}

// GoalRunActivator hands a committed Goal successor to the existing task-run activation path.
type GoalRunActivator interface {
	ActivateGoalRun(context.Context, task.Run)
}

// GoalRunActivatorFunc adapts a function to GoalRunActivator.
type GoalRunActivatorFunc func(context.Context, task.Run)

// ActivateGoalRun implements GoalRunActivator.
func (f GoalRunActivatorFunc) ActivateGoalRun(ctx context.Context, run task.Run) {
	if f != nil {
		f(ctx, run)
	}
}

// GoalControlStore is the optional atomic re-entry extension implemented by Loop stores.
type GoalControlStore interface {
	LoadAwaitingGoalControl(context.Context, WorkspaceID, RunID) (GoalControlState, bool, error)
	ReactivateGoalRun(context.Context, GoalReactivationRequest) (GoalReactivationResult, error)
}
