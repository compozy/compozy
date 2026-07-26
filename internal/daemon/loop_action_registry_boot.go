package daemon

import (
	"fmt"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/gate"
)

func newBootLoopActionRegistry(
	store taskStore,
	state *bootState,
	globalWorkspacePath string,
	toolRegistry lazyToolRegistry,
	policyGate *loopSessionPolicyGate,
	gateEvaluator gate.GateEvaluator,
	judgeExecutions *loopJudgeExecutionRegistry,
) (*looppkg.ActionRegistry, error) {
	actionBinder := &loopActionSessionBinder{
		sessions:            state.sessions,
		globalWorkspacePath: globalWorkspacePath,
		policyGate:          policyGate,
	}
	actionOptions := []looppkg.ActionRegistryOption{
		looppkg.WithActionSessionBinder(actionBinder),
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
		goalRuntime := newLoopGoalProductionRuntime(
			goalStore,
			state.sessions,
			globalWorkspacePath,
			policyGate,
			time.Now,
			state.logger,
		)
		actionOptions[0] = looppkg.WithActionSessionBinder(goalRuntime)
		goalOption, err := composeLoopGoalExecutor(
			goalStore,
			goalRuntime,
			gateEvaluator,
			judgeExecutions,
		)
		if err != nil {
			return nil, fmt.Errorf("daemon: compose Goal executor: %w", err)
		}
		actionOptions = append(actionOptions, goalOption)
	} else if _, supportsGoalExecutor := store.(loopGoalExecutorStore); supportsGoalExecutor && state.sessions != nil {
		return nil, fmt.Errorf("%w: Goal production store ports are incomplete", looppkg.ErrActionDependencyMissing)
	}
	if conversations, ok := state.registry.(looppkg.ChannelResultConversationStore); ok {
		actionOptions = append(actionOptions, looppkg.WithActionChannelResultStore(conversations))
	}
	return looppkg.NewActionRegistry(toolRegistry, actionOptions...)
}
