package callworkflows

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

const toolID = "cp__call_workflows"

func init() { //nolint:gochecknoinits // register builtin provider at startup
	native.RegisterProvider(Definition)
}

// Definition exposes the builtin definition for cp__call_workflows.
func Definition(env toolenv.Environment) builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:           toolID,
		Description:  "Execute multiple workflows in parallel.",
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
	if env == nil || env.WorkflowExecutor() == nil {
		return nil, status, 0, builtin.CodeInternal, builtin.Internal(errors.New("workflow executor unavailable"), nil)
	}
	log := logger.FromContext(ctx)
	batchCfg := resolveCallWorkflowsConfig(ctx)
	if !batchCfg.Enabled {
		return nil, status, 0, builtin.CodePermissionDenied, builtin.PermissionDenied(
			errors.New("call workflows tool disabled"),
			nil,
		)
	}
	input, code, err := decodeHandlerInput(payload)
	if err != nil {
		return nil, status, 0, code, err
	}
	plans, code, err := buildWorkflowPlans(input.Workflows, batchCfg)
	if err != nil {
		return nil, status, 0, code, err
	}
	requestID := builtin.RequestIDFromContext(ctx)
	logParallelWorkflowStart(log, requestID, len(plans), batchCfg.MaxConcurrent)
	results := executeWorkflowsParallel(ctx, env, plans, batchCfg.MaxConcurrent)
	summary := summarizeResults(results, time.Since(start).Milliseconds())
	output := buildHandlerOutput(results, summary)
	logParallelWorkflowComplete(log, requestID, summary)
	encoded, err := json.Marshal(output)
	if err != nil {
		log.Warn("Failed to encode call_workflows output", "request_id", requestID, "error", err)
		return output, builtin.StatusSuccess, 0, "", nil
	}
	return output, builtin.StatusSuccess, len(encoded), "", nil
}

func resolveCallWorkflowsConfig(ctx context.Context) config.NativeCallWorkflowsConfig {
	defaults := config.DefaultNativeToolsConfig()
	if appCfg := config.FromContext(ctx); appCfg != nil {
		return appCfg.Runtime.NativeTools.CallWorkflows
	}
	return defaults.CallWorkflows
}

func logParallelWorkflowStart(log logger.Logger, requestID string, workflowCount, maxConcurrent int) {
	log.Info(
		"Parallel workflow execution requested",
		"request_id", requestID,
		"workflow_count", workflowCount,
		"max_concurrent", maxConcurrent,
	)
}

func logParallelWorkflowComplete(log logger.Logger, requestID string, summary executionSummary) {
	log.Info(
		"Parallel workflow execution complete",
		"request_id", requestID,
		"total", summary.TotalCount,
		"success", summary.SuccessCount,
		"failed", summary.FailureCount,
		"duration_ms", summary.TotalDuration,
	)
}
