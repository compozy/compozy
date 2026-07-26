package daemon

import (
	"context"
	"fmt"
	"strings"
	"sync"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/tools"
)

type loopGoalRuntimePorts interface {
	looppkg.ManagedActionSessionBinder
	goalpkg.ContextHealth
	goalpkg.PromptRecovery
}

type loopGoalExecutorStore interface {
	goalpkg.ExecutorStore
	goalpkg.BudgetGuard
	looppkg.Store
}

type loopGoalProductionStore interface {
	loopGoalExecutorStore
	loopGoalPromptRecoveryStore
	goalpkg.BindingStore
	store.SessionCreationStore
}

type loopGoalRuntime struct {
	*loopActionSessionBinder
	*loopGoalContextRuntime
	*loopGoalPromptRecovery
}

var _ loopGoalRuntimePorts = (*loopGoalRuntime)(nil)

type loopManagedInputLifecycleInstaller interface {
	SetManagedInputLifecycle(session.ManagedInputLifecycle)
}

func composeLoopGoalExecutor(
	store loopGoalExecutorStore,
	runtime loopGoalRuntimePorts,
	evaluator gate.GateEvaluator,
	executions *loopJudgeExecutionRegistry,
) (looppkg.ActionRegistryOption, error) {
	if store == nil || runtime == nil || evaluator == nil || executions == nil {
		return nil, fmt.Errorf("%w: Goal runtime ports are incomplete", looppkg.ErrActionDependencyMissing)
	}
	executor, err := goalpkg.NewExecutor(goalpkg.Dependencies{
		Store:    store,
		Binder:   runtime,
		Judge:    &loopGoalJudgeEvaluator{store: store, evaluator: evaluator, executions: executions},
		Budget:   store,
		Context:  runtime,
		Recovery: runtime,
	})
	if err != nil {
		return nil, err
	}
	return looppkg.WithActionGoalExecutor(executor), nil
}

type loopGoalJudgeEvaluator struct {
	store      looppkg.Store
	evaluator  gate.GateEvaluator
	executions *loopJudgeExecutionRegistry
}

func (e *loopGoalJudgeEvaluator) EvaluateGoal(
	ctx context.Context,
	req goalpkg.JudgeRequest,
) (goalpkg.JudgeResult, error) {
	if e == nil || e.store == nil || e.evaluator == nil || e.executions == nil {
		return goalpkg.JudgeResult{}, fmt.Errorf(
			"%w: Goal judge evaluator is unavailable",
			looppkg.ErrActionDependencyMissing,
		)
	}
	release, err := e.executions.reserve(req.AttemptID)
	if err != nil {
		return goalpkg.JudgeResult{}, err
	}
	defer release()
	run, err := e.store.GetLoopRun(ctx, req.Key.WorkspaceID, req.Key.LoopRunID)
	if err != nil {
		return goalpkg.JudgeResult{}, err
	}
	snapshot, err := e.store.GetLoopDefinitionSnapshot(ctx, run.WorkspaceID, run.DefinitionDigest)
	if err != nil {
		return goalpkg.JudgeResult{}, err
	}
	resolved, err := looppkg.LoadExecutedDefinitionSnapshot(snapshot.Definition, run.DefinitionDigest)
	if err != nil {
		return goalpkg.JudgeResult{}, fmt.Errorf("daemon: load Goal judge snapshot: %w", err)
	}
	usage := &loopGoalJudgeUsage{}
	verdict, err := e.evaluator.Evaluate(ctx, gate.Gate{
		ID:            string(req.Key.NodeID) + ":goal-judge",
		Criteria:      append([]dsl.GateCriterion(nil), req.Criteria...),
		VerdictPolicy: dsl.VerdictPolicyReviseUntilClean,
	}, gate.GateInput{
		LoopRunID:              string(run.ID),
		Placement:              gate.PlacementInBody,
		Contract:               new(resolved.Definition.Contract),
		TemplateData:           map[string]any{"goal_turn": req.Turn},
		Revision:               req.Turn - 1,
		BrokenJudgeStreakLimit: gate.DefaultBrokenJudgeStreakLimit,
		ToolScope: tools.Scope{
			WorkspaceID: string(req.Key.WorkspaceID),
			ActorKind:   harnessSummaryDefaultAgentName,
		},
		ToolCallCorrelationID: strings.TrimSpace(req.AttemptID),
		JudgeModel:            strings.TrimSpace(resolved.EffectiveConfig.ModelDefaults.Judge),
		JudgeUsageReporter:    usage,
		JudgeEvidence: gate.JudgeEvidence{
			Text:       req.Result.Text,
			Structured: append([]byte(nil), req.Result.Structured...),
		},
		NetworkParticipation: new(run.NetworkSpecSnapshot()),
	})
	if err != nil {
		return goalpkg.JudgeResult{}, err
	}
	tokensUsed, tokensReported := usage.snapshot()
	return goalpkg.JudgeResult{
		Verdict: verdict, TokensUsed: tokensUsed, TokensReported: tokensReported,
	}, nil
}

type loopGoalJudgeUsage struct {
	mu       sync.Mutex
	tokens   int64
	reported bool
}

func (u *loopGoalJudgeUsage) ReportJudgeTokensUsed(tokens int64) {
	if u == nil || tokens < 0 {
		return
	}
	u.mu.Lock()
	u.tokens = max(u.tokens, tokens)
	u.reported = true
	u.mu.Unlock()
}

func (u *loopGoalJudgeUsage) snapshot() (int64, bool) {
	if u == nil {
		return 0, false
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.tokens, u.reported
}
