package callagents

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/semaphore"
)

type agentPlan struct {
	index      int
	request    toolenv.AgentRequest
	userConfig AgentExecutionRequest
}

func executeAgentsParallel(
	ctx context.Context,
	env toolenv.Environment,
	plans []agentPlan,
	maxConcurrent int,
) []AgentExecutionResult {
	if len(plans) == 0 {
		return nil
	}
	results := make([]AgentExecutionResult, len(plans))
	sem := semaphore.NewWeighted(int64(effectiveMaxConcurrent(maxConcurrent)))
	var wg sync.WaitGroup
	for i := range plans {
		plan := &plans[i]
		wg.Go(func() {
			executePlan(ctx, env, sem, plan, results)
		})
	}
	wg.Wait()
	return results
}

func effectiveMaxConcurrent(limit int) int {
	if limit <= 0 {
		return 1
	}
	return limit
}

func executePlan(
	ctx context.Context,
	env toolenv.Environment,
	sem *semaphore.Weighted,
	plan *agentPlan,
	results []AgentExecutionResult,
) {
	log := logger.FromContext(ctx)
	start := time.Now()
	log.Info(
		"Agent execution starting",
		"agent_id", plan.userConfig.AgentID,
		"action_id", plan.userConfig.ActionID,
	)
	result := runAgent(ctx, env, sem, plan)
	results[plan.index] = result
	recordAgentStep(ctx, plan.userConfig, &result, time.Since(start))
	log.Info(
		"Agent execution finished",
		"agent_id", plan.userConfig.AgentID,
		"action_id", plan.userConfig.ActionID,
		"success", result.Success,
		"duration_ms", result.DurationMs,
		"error_code", errorCodeForResult(&result),
	)
}

func recordAgentStep(
	ctx context.Context,
	config AgentExecutionRequest,
	result *AgentExecutionResult,
	duration time.Duration,
) {
	status := builtin.StatusFailure
	if result != nil && result.Success {
		status = builtin.StatusSuccess
	}
	builtin.RecordStep(
		ctx,
		toolID,
		stepTypeAgent,
		status,
		duration,
		attribute.String("agent_id", config.AgentID),
		attribute.String("action_id", config.ActionID),
	)
}

func runAgent(
	ctx context.Context,
	env toolenv.Environment,
	sem *semaphore.Weighted,
	plan *agentPlan,
) AgentExecutionResult {
	result := AgentExecutionResult{
		AgentID:  plan.userConfig.AgentID,
		ActionID: plan.userConfig.ActionID,
	}
	start := time.Now()
	defer handleAgentPanic(ctx, plan.userConfig, start, &result)
	if err := sem.Acquire(ctx, 1); err != nil {
		return applySemaphoreFailure(&result, err)
	}
	defer sem.Release(1)
	executor := env.AgentExecutor()
	if executor == nil {
		return applyInternalFailure(&result, errors.New("agent executor unavailable"))
	}
	agentCtx, cancel := deriveAgentContext(ctx, plan.request.Timeout)
	defer cancel()
	res, err := executor.ExecuteAgent(agentCtx, plan.request)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		return applyExecutionFailure(&result, err, agentCtx.Err(), duration)
	}
	populateSuccess(&result, res, duration)
	return result
}

func deriveAgentContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}

func handleAgentPanic(
	ctx context.Context,
	config AgentExecutionRequest,
	start time.Time,
	result *AgentExecutionResult,
) {
	if r := recover(); r != nil {
		log := logger.FromContext(ctx)
		result.Success = false
		result.Error = &ErrorDetails{
			Message: fmt.Sprintf("panic: %v", r),
			Code:    builtin.CodeInternal,
		}
		result.DurationMs = time.Since(start).Milliseconds()
		log.Error(
			"Recovered panic while executing agent",
			"agent_id", config.AgentID,
			"action_id", config.ActionID,
			"error", r,
		)
	}
}

func applySemaphoreFailure(result *AgentExecutionResult, err error) AgentExecutionResult {
	code := builtin.CodeInternal
	if errors.Is(err, context.DeadlineExceeded) {
		code = builtin.CodeDeadlineExceeded
	}
	result.Error = &ErrorDetails{Message: err.Error(), Code: code}
	result.DurationMs = 0
	return *result
}

func applyInternalFailure(result *AgentExecutionResult, err error) AgentExecutionResult {
	result.Error = &ErrorDetails{Message: err.Error(), Code: builtin.CodeInternal}
	return *result
}

func applyExecutionFailure(
	result *AgentExecutionResult,
	execErr error,
	ctxErr error,
	duration int64,
) AgentExecutionResult {
	code := builtin.CodeInternal
	if ctxErr != nil && errors.Is(ctxErr, context.DeadlineExceeded) {
		code = builtin.CodeDeadlineExceeded
	} else {
		var cerr *core.Error
		if errors.As(execErr, &cerr) {
			code = cerr.Code
		}
	}
	if errors.Is(execErr, context.DeadlineExceeded) {
		code = builtin.CodeDeadlineExceeded
	}
	result.Error = &ErrorDetails{Message: execErr.Error(), Code: code}
	result.DurationMs = duration
	return *result
}

func populateSuccess(result *AgentExecutionResult, res *toolenv.AgentResult, duration int64) {
	result.Success = true
	result.DurationMs = duration
	if res == nil {
		return
	}
	result.ExecID = res.ExecID.String()
	if res.Output == nil {
		return
	}
	if copied, err := core.DeepCopy(*res.Output); err == nil {
		result.Response = copied
		return
	}
	result.Response = *res.Output
}

func errorCodeForResult(result *AgentExecutionResult) string {
	if result == nil || result.Error == nil {
		return ""
	}
	return result.Error.Code
}
