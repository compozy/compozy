package callworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/native"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const toolID = "cp__call_workflow"

func init() { //nolint:gochecknoinits // builtin registration on package load
	native.RegisterProvider(Definition)
}

// Definition exposes the builtin definition for cp__call_workflow.
func Definition(env toolenv.Environment) builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:           toolID,
		Description:  "Execute a single workflow synchronously.",
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
		result, stat, written, code, err := processRequest(ctx, env, payload)
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
) (core.Output, string, int, string, error) {
	executor, code, err := workflowExecutorFromEnv(env)
	if err != nil {
		return nil, builtin.StatusFailure, 0, code, err
	}
	workflowCfg := resolveCallWorkflowConfig(ctx)
	if !workflowCfg.Enabled {
		return nil, builtin.StatusFailure, 0, builtin.CodePermissionDenied, builtin.PermissionDenied(
			errors.New("call workflow tool disabled"),
			nil,
		)
	}
	input, code, err := decodeHandlerInput(payload)
	if err != nil {
		return nil, builtin.StatusFailure, 0, code, err
	}
	req, code, err := buildWorkflowRequest(workflowCfg, input)
	if err != nil {
		return nil, builtin.StatusFailure, 0, code, err
	}
	result, duration, code, err := performWorkflowExecution(ctx, executor, workflowCfg, req)
	if err != nil {
		return nil, builtin.StatusFailure, 0, code, err
	}
	output := buildHandlerOutput(req, result, duration)
	encoded, err := json.Marshal(output)
	if err != nil {
		logger.FromContext(ctx).Error("Failed to encode call_workflow output", "error", err)
		return nil,
			builtin.StatusFailure,
			0,
			builtin.CodeInternal,
			builtin.Internal(
				fmt.Errorf("failed to encode call_workflow output: %w", err),
				nil,
			)
	}
	return output, builtin.StatusSuccess, len(encoded), "", nil
}

func workflowExecutorFromEnv(env toolenv.Environment) (toolenv.WorkflowExecutor, string, error) {
	if env == nil || env.WorkflowExecutor() == nil {
		return nil, builtin.CodeInternal, builtin.Internal(errors.New("workflow executor unavailable"), nil)
	}
	return env.WorkflowExecutor(), "", nil
}

func resolveCallWorkflowConfig(ctx context.Context) config.NativeCallWorkflowConfig {
	cfg := config.DefaultNativeToolsConfig()
	if appCfg := config.FromContext(ctx); appCfg != nil {
		return appCfg.Runtime.NativeTools.CallWorkflow
	}
	return cfg.CallWorkflow
}

func performWorkflowExecution(
	ctx context.Context,
	executor toolenv.WorkflowExecutor,
	cfg config.NativeCallWorkflowConfig,
	req toolenv.WorkflowRequest,
) (*toolenv.WorkflowResult, time.Duration, string, error) {
	timeout := req.Timeout
	if timeout <= 0 && cfg.DefaultTimeout > 0 {
		timeout = cfg.DefaultTimeout
	}
	execCtx, cancel := deriveHandlerExecutionContext(ctx, timeout)
	defer cancel()
	start := time.Now()
	result, execErr := executor.ExecuteWorkflow(execCtx, req)
	if execErr != nil {
		code, err := resolveExecutionError(execErr)
		return nil, 0, code, err
	}
	return result, time.Since(start), "", nil
}

func deriveHandlerExecutionContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

func resolveExecutionError(execErr error) (string, error) {
	if execErr == nil {
		return "", nil
	}
	if errors.Is(execErr, context.DeadlineExceeded) {
		return builtin.CodeDeadlineExceeded, builtin.DeadlineExceeded(execErr, nil)
	}
	var cerr *core.Error
	if errors.As(execErr, &cerr) {
		return cerr.Code, cerr
	}
	return builtin.CodeInternal, builtin.Internal(execErr, nil)
}
