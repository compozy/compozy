package callworkflows

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

const stepTypeWorkflow = "workflow_execution"

func executeWorkflowsParallel(
	ctx context.Context,
	env toolenv.Environment,
	plans []workflowPlan,
	maxConcurrent int,
) []WorkflowExecutionResult {
	if len(plans) == 0 {
		return nil
	}
	results := make([]WorkflowExecutionResult, len(plans))
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
	plan *workflowPlan,
	results []WorkflowExecutionResult,
) {
	log := logger.FromContext(ctx)
	start := time.Now()
	log.Info(
		"Workflow execution starting",
		"workflow_id", plan.userConfig.WorkflowID,
	)
	result := runWorkflow(ctx, env, sem, plan)
	results[plan.index] = result
	recordWorkflowStep(ctx, plan.userConfig, &result, time.Since(start))
	log.Info(
		"Workflow execution finished",
		"workflow_id", plan.userConfig.WorkflowID,
		"success", result.Success,
		"status", result.Status,
		"duration_ms", result.DurationMs,
		"error_code", errorCodeForResult(&result),
	)
}

func recordWorkflowStep(
	ctx context.Context,
	config WorkflowExecutionRequest,
	result *WorkflowExecutionResult,
	duration time.Duration,
) {
	status := builtin.StatusFailure
	if result != nil && result.Success {
		status = builtin.StatusSuccess
	}
	builtin.RecordStep(
		ctx,
		toolID,
		stepTypeWorkflow,
		status,
		duration,
		attribute.String("workflow_id", config.WorkflowID),
	)
}

func runWorkflow(
	ctx context.Context,
	env toolenv.Environment,
	sem *semaphore.Weighted,
	plan *workflowPlan,
) WorkflowExecutionResult {
	result := WorkflowExecutionResult{WorkflowID: plan.userConfig.WorkflowID, Status: string(core.StatusFailed)}
	start := time.Now()
	defer handleWorkflowPanic(ctx, plan.userConfig, start, &result)
	if err := sem.Acquire(ctx, 1); err != nil {
		return applySemaphoreFailure(&result, err)
	}
	defer sem.Release(1)
	executor := env.WorkflowExecutor()
	if executor == nil {
		return applyInternalFailure(&result, errors.New("workflow executor unavailable"))
	}
	execCtx, cancel := deriveWorkflowContext(ctx, plan.request.Timeout)
	defer cancel()
	res, err := executor.ExecuteWorkflow(execCtx, plan.request)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		return applyExecutionFailure(&result, err, execCtx.Err(), duration)
	}
	populateSuccess(&result, res, duration)
	return result
}

func deriveWorkflowContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}

func handleWorkflowPanic(
	ctx context.Context,
	config WorkflowExecutionRequest,
	start time.Time,
	result *WorkflowExecutionResult,
) {
	if r := recover(); r != nil {
		log := logger.FromContext(ctx)
		result.Success = false
		result.Status = string(core.StatusFailed)
		result.Error = &ErrorDetails{Message: fmt.Sprintf("panic: %v", r), Code: builtin.CodeInternal}
		result.DurationMs = time.Since(start).Milliseconds()
		log.Error(
			"Recovered panic while executing workflow",
			"workflow_id", config.WorkflowID,
			"error", r,
		)
	}
}

func applySemaphoreFailure(result *WorkflowExecutionResult, err error) WorkflowExecutionResult {
	code := builtin.CodeInternal
	status := string(core.StatusFailed)
	if errors.Is(err, context.DeadlineExceeded) {
		code = builtin.CodeDeadlineExceeded
		status = string(core.StatusTimedOut)
	}
	result.Error = &ErrorDetails{Message: err.Error(), Code: code}
	result.Status = status
	result.DurationMs = 0
	return *result
}

func applyInternalFailure(result *WorkflowExecutionResult, err error) WorkflowExecutionResult {
	result.Error = &ErrorDetails{Message: err.Error(), Code: builtin.CodeInternal}
	result.Status = string(core.StatusFailed)
	return *result
}

func applyExecutionFailure(
	result *WorkflowExecutionResult,
	execErr error,
	ctxErr error,
	duration int64,
) WorkflowExecutionResult {
	code := builtin.CodeInternal
	status := string(core.StatusFailed)
	switch {
	case ctxErr != nil && errors.Is(ctxErr, context.DeadlineExceeded):
		code = builtin.CodeDeadlineExceeded
		status = string(core.StatusTimedOut)
	case errors.Is(execErr, context.DeadlineExceeded):
		code = builtin.CodeDeadlineExceeded
		status = string(core.StatusTimedOut)
	default:
		var cerr *core.Error
		if errors.As(execErr, &cerr) {
			code = cerr.Code
		}
	}
	result.Error = &ErrorDetails{Message: execErr.Error(), Code: code}
	result.Status = status
	result.DurationMs = duration
	result.Success = false
	return *result
}

func populateSuccess(result *WorkflowExecutionResult, res *toolenv.WorkflowResult, duration int64) {
	result.Success = true
	result.Status = string(core.StatusSuccess)
	result.DurationMs = duration
	if res == nil {
		return
	}
	result.WorkflowExecID = res.WorkflowExecID.String()
	if res.Status != "" {
		result.Status = res.Status
		if core.StatusType(res.Status) != core.StatusSuccess {
			result.Success = false
		}
	}
	if res.Output == nil {
		return
	}
	if clone, err := res.Output.Clone(); err == nil && clone != nil {
		result.Output = *clone
		return
	}
	result.Output = *res.Output
}

func errorCodeForResult(result *WorkflowExecutionResult) string {
	if result == nil || result.Error == nil {
		return ""
	}
	return result.Error.Code
}

type executionSummary struct {
	TotalCount    int
	SuccessCount  int
	FailureCount  int
	TotalDuration int64
}

func summarizeResults(results []WorkflowExecutionResult, elapsedMs int64) executionSummary {
	summary := executionSummary{
		TotalCount:    len(results),
		TotalDuration: elapsedMs,
	}
	for i := range results {
		if results[i].Success {
			summary.SuccessCount++
		} else {
			summary.FailureCount++
		}
	}
	return summary
}

func buildHandlerOutput(results []WorkflowExecutionResult, summary executionSummary) core.Output {
	return core.Output{
		"results":           results,
		"total_count":       summary.TotalCount,
		"success_count":     summary.SuccessCount,
		"failure_count":     summary.FailureCount,
		"total_duration_ms": summary.TotalDuration,
	}
}
