package calltasks

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/semaphore"
)

const (
	stepTypeTask = "task_execution"
)

type taskPlan struct {
	index      int
	request    toolenv.TaskRequest
	userConfig TaskExecutionRequest
}

func executeTasksParallel(
	ctx context.Context,
	env toolenv.Environment,
	plans []taskPlan,
	maxConcurrent int,
) []TaskExecutionResult {
	if len(plans) == 0 {
		return nil
	}
	results := make([]TaskExecutionResult, len(plans))
	effective := effectiveMaxConcurrent(maxConcurrent, len(plans))
	sem := semaphore.NewWeighted(int64(effective))
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

func effectiveMaxConcurrent(limit int, planCount int) int {
	if planCount <= 0 {
		return 0
	}
	if limit <= 0 {
		return 1
	}
	if limit > planCount {
		return planCount
	}
	return limit
}

func executePlan(
	ctx context.Context,
	env toolenv.Environment,
	sem *semaphore.Weighted,
	plan *taskPlan,
	results []TaskExecutionResult,
) {
	log := logger.FromContext(ctx)
	start := time.Now()
	log.Info(
		"Task execution starting",
		"task_id", plan.userConfig.TaskID,
	)
	result := runTask(ctx, env, sem, plan)
	results[plan.index] = result
	recordTaskStep(ctx, plan.userConfig, &result, time.Since(start))
	log.Info(
		"Task execution finished",
		"task_id", plan.userConfig.TaskID,
		"success", result.Success,
		"duration_ms", result.DurationMs,
		"error_code", errorCodeForResult(&result),
	)
}

func recordTaskStep(
	ctx context.Context,
	config TaskExecutionRequest,
	result *TaskExecutionResult,
	duration time.Duration,
) {
	status := builtin.StatusFailure
	if result != nil && result.Success {
		status = builtin.StatusSuccess
	}
	builtin.RecordStep(
		ctx,
		toolID,
		stepTypeTask,
		status,
		duration,
		attribute.String("task_id", config.TaskID),
	)
}

func runTask(
	ctx context.Context,
	env toolenv.Environment,
	sem *semaphore.Weighted,
	plan *taskPlan,
) TaskExecutionResult {
	result := TaskExecutionResult{TaskID: plan.userConfig.TaskID}
	start := time.Now()
	defer handleTaskPanic(ctx, plan.userConfig, start, &result)
	if err := sem.Acquire(ctx, 1); err != nil {
		elapsed := time.Since(start).Milliseconds()
		return applySemaphoreFailure(&result, err, elapsed)
	}
	defer sem.Release(1)
	executor := env.TaskExecutor()
	if executor == nil {
		return applyInternalFailure(&result, errors.New("task executor unavailable"))
	}
	taskCtx, cancel := deriveTaskContext(ctx, plan.request.Timeout)
	defer cancel()
	res, err := executor.ExecuteTask(taskCtx, plan.request)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		return applyExecutionFailure(&result, err, taskCtx.Err(), duration)
	}
	populateSuccess(&result, res, duration)
	return result
}

func deriveTaskContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}

func handleTaskPanic(
	ctx context.Context,
	config TaskExecutionRequest,
	start time.Time,
	result *TaskExecutionResult,
) {
	if r := recover(); r != nil {
		log := logger.FromContext(ctx)
		stack := debug.Stack()
		result.Success = false
		result.Error = &ErrorDetails{
			Message: fmt.Sprintf("panic: %v; stack: %s", r, stack),
			Code:    builtin.CodeInternal,
		}
		result.DurationMs = time.Since(start).Milliseconds()
		log.Error(
			"Recovered panic while executing task",
			"task_id", config.TaskID,
			"error", r,
			"stack", string(stack),
		)
	}
}

func applySemaphoreFailure(result *TaskExecutionResult, err error, duration int64) TaskExecutionResult {
	code := builtin.CodeInternal
	if errors.Is(err, context.DeadlineExceeded) {
		code = builtin.CodeDeadlineExceeded
	}
	result.Error = &ErrorDetails{Message: err.Error(), Code: code}
	result.DurationMs = duration
	return *result
}

func applyInternalFailure(result *TaskExecutionResult, err error) TaskExecutionResult {
	result.Error = &ErrorDetails{Message: err.Error(), Code: builtin.CodeInternal}
	return *result
}

func applyExecutionFailure(
	result *TaskExecutionResult,
	execErr error,
	ctxErr error,
	duration int64,
) TaskExecutionResult {
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

func populateSuccess(result *TaskExecutionResult, res *toolenv.TaskResult, duration int64) {
	result.Success = true
	result.DurationMs = duration
	if res == nil {
		return
	}
	result.ExecID = res.ExecID.String()
	if res.Output == nil {
		return
	}
	if clone, err := res.Output.Clone(); err == nil && clone != nil {
		result.Output = *clone
		return
	}
	result.Output = *res.Output
}

func errorCodeForResult(result *TaskExecutionResult) string {
	if result == nil || result.Error == nil {
		return ""
	}
	return result.Error.Code
}
