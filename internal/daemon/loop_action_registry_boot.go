package daemon

import (
	"fmt"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/session"
)

// newBootLoopActionRegistry wires Loop actions to daemon-owned sessions, judges, and worktree leases.
func newBootLoopActionRegistry(
	store taskStore,
	state *bootState,
	toolRegistry lazyToolRegistry,
	policyGate *loopSessionPolicyGate,
	gateEvaluator gate.GateEvaluator,
	judgeExecutions *loopJudgeExecutionRegistry,
) (*looppkg.ActionRegistry, error) {
	worktrees := executionWorktreesForState(state)
	traces := installLoopRunTraces(state)
	actionBinder := &loopActionSessionBinder{
		sessions: state.sessions,
		traces:   traces,

		policyGate: policyGate,
		worktrees:  worktrees,
	}
	actionOptions := []looppkg.ActionRegistryOption{
		looppkg.WithActionDefaultMaxResultBytes(state.cfg.Tools.DefaultMaxResultBytes),
		looppkg.WithActionSessionBinder(actionBinder),
		looppkg.WithActionToolWorkspaceRootResolver(&loopActionToolWorkspaceRootResolver{
			worktrees: worktrees,
		}),
		looppkg.WithActionLoopStarter(lazyLoopStarter{current: func() looppkg.ActionLoopStarter {
			if state == nil {
				return nil
			}
			starter, ok := state.deps.Loops.(looppkg.ActionLoopStarter)
			if !ok {
				return nil
			}
			return starter
		}}),
	}
	if goalStore, ok := store.(loopGoalProductionStore); ok && state.sessions != nil {
		productionRuntime := newLoopGoalProductionRuntime(
			goalStore,
			state.sessions,
			policyGate,
			time.Now,
			state.logger,
			worktrees,
		)
		productionRuntime.traces = traces
		if manager, ok := state.sessions.(*session.Manager); ok {
			productionRuntime.waitSources = &sessionWorkSources{state: state, manager: manager, now: time.Now}
		}
		goalRuntime := withGoalToolProjection(productionRuntime, goalToolProjectionInvalidator(state))
		installGoalToolProjectionObserver(state)
		actionOptions[1] = looppkg.WithActionSessionBinder(goalRuntime)
		goalOption, err := composeLoopGoalExecutor(
			goalStore,
			goalRuntime,
			gateEvaluator,
			judgeExecutions,
			&loopActionToolWorkspaceRootResolver{worktrees: worktrees},
		)
		if err != nil {
			return nil, fmt.Errorf("daemon: compose Goal executor: %w", err)
		}
		actionOptions = append(actionOptions, goalOption)
	} else if _, supportsGoalExecutor := store.(loopGoalExecutorStore); supportsGoalExecutor && state.sessions != nil {
		return nil, fmt.Errorf("%w: Goal production store ports are incomplete", looppkg.ErrActionDependencyMissing)
	}
	return looppkg.NewActionRegistry(toolRegistry, actionOptions...)
}

// installLoopRunTraces registers the loop.run span registry with the Loop lifecycle observers.
func installLoopRunTraces(state *bootState) *looppkg.RunTraces {
	traces := &looppkg.RunTraces{}
	if state != nil && state.notifier != nil {
		state.notifier.AddLoopStartedObserver(traces)
		state.notifier.AddLoopTerminalObserver(traces)
	}
	return traces
}
