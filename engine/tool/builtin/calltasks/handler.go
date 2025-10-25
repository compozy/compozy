package calltasks

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/native"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const toolID = "cp__call_tasks"

func init() { //nolint:gochecknoinits // register builtin provider at startup
	native.RegisterProvider(Definition)
}

// Definition exposes the builtin definition for cp__call_tasks.
func Definition(env toolenv.Environment) builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:           toolID,
		Description:  "Execute multiple tasks in parallel.",
		InputSchema:  &inputSchema,
		OutputSchema: &outputSchema,
		Handler:      newHandler(env),
	}
}

func newHandler(env toolenv.Environment) builtin.Handler {
	return func(ctx context.Context, payload map[string]any) (core.Output, error) {
		start := time.Now()
		status := builtin.StatusFailure
		errorCode := ""
		bytesWritten := 0
		var output core.Output
		defer func() {
			builtin.RecordInvocation(
				ctx,
				toolID,
				builtin.RequestIDFromContext(ctx),
				status,
				time.Since(start),
				bytesWritten,
				errorCode,
			)
		}()
		result, stat, written, code, err := processRequest(ctx, env, payload, start)
		if err != nil {
			errorCode = code
			return nil, err
		}
		status = stat
		bytesWritten = written
		output = result
		return output, nil
	}
}

func processRequest(
	ctx context.Context,
	env toolenv.Environment,
	payload map[string]any,
	start time.Time,
) (core.Output, string, int, string, error) {
	status := builtin.StatusFailure
	if env == nil || env.TaskExecutor() == nil {
		return nil, status, 0, builtin.CodeInternal, builtin.Internal(errors.New("task executor unavailable"), nil)
	}
	log := logger.FromContext(ctx)
	nativeCfg := config.DefaultNativeToolsConfig()
	if appCfg := config.FromContext(ctx); appCfg != nil {
		nativeCfg = appCfg.Runtime.NativeTools
	}
	batchCfg := nativeCfg.CallTasks
	if !batchCfg.Enabled {
		return nil, status, 0, builtin.CodePermissionDenied, builtin.PermissionDenied(
			errors.New("call tasks tool disabled"),
			nil,
		)
	}
	input, code, err := decodeHandlerInput(payload)
	if err != nil {
		return nil, status, 0, code, err
	}
	plans, code, err := buildTaskPlans(input.Tasks, batchCfg)
	if err != nil {
		return nil, status, 0, code, err
	}
	log.Info(
		"Parallel task execution requested",
		"task_count", len(plans),
		"max_concurrent", batchCfg.MaxConcurrent,
	)
	results := executeTasksParallel(ctx, env, plans, batchCfg.MaxConcurrent)
	summary := summarizeResults(results, time.Since(start).Milliseconds())
	output := buildHandlerOutput(results, summary)
	log.Info(
		"Parallel task execution complete",
		"total", summary.TotalCount,
		"success", summary.SuccessCount,
		"failed", summary.FailureCount,
		"duration_ms", summary.TotalDuration,
	)
	encoded, err := json.Marshal(output)
	if err != nil {
		log.Warn("Failed to encode call_tasks output", "error", err)
		return output, builtin.StatusSuccess, 0, "", nil
	}
	return output, builtin.StatusSuccess, len(encoded), "", nil
}

type executionSummary struct {
	TotalCount    int
	SuccessCount  int
	FailureCount  int
	TotalDuration int64
}

func summarizeResults(results []TaskExecutionResult, elapsedMs int64) executionSummary {
	summary := executionSummary{
		TotalCount:    len(results),
		TotalDuration: elapsedMs,
	}
	for _, result := range results {
		if result.Success {
			summary.SuccessCount++
		} else {
			summary.FailureCount++
		}
	}
	return summary
}

func buildHandlerOutput(results []TaskExecutionResult, summary executionSummary) core.Output {
	return core.Output{
		"results":           results,
		"total_count":       summary.TotalCount,
		"success_count":     summary.SuccessCount,
		"failure_count":     summary.FailureCount,
		"total_duration_ms": summary.TotalDuration,
	}
}
