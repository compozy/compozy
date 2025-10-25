package callworkflow

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
	status := builtin.StatusFailure
	if env == nil || env.WorkflowExecutor() == nil {
		return nil, status, 0, builtin.CodeInternal, builtin.Internal(errors.New("workflow executor unavailable"), nil)
	}
	nativeCfg := config.DefaultNativeToolsConfig()
	if appCfg := config.FromContext(ctx); appCfg != nil {
		nativeCfg = appCfg.Runtime.NativeTools
	}
	workflowCfg := nativeCfg.CallWorkflow
	if !workflowCfg.Enabled {
		return nil, status, 0, builtin.CodePermissionDenied, builtin.PermissionDenied(
			errors.New("call workflow tool disabled"),
			nil,
		)
	}
	input, code, err := decodeHandlerInput(payload)
	if err != nil {
		return nil, status, 0, code, err
	}
	req, code, err := buildWorkflowRequest(workflowCfg, input)
	if err != nil {
		return nil, status, 0, code, err
	}
	executor := env.WorkflowExecutor()
	execStart := time.Now()
	result, execErr := executor.ExecuteWorkflow(ctx, req)
	duration := time.Since(execStart)
	if execErr != nil {
		code, err := resolveExecutionError(execErr)
		return nil, status, 0, code, err
	}
	output := buildHandlerOutput(req, result, duration)
	encoded, err := json.Marshal(output)
	if err != nil {
		logger.FromContext(ctx).Warn("Failed to encode call_workflow output", "error", err)
		return output, builtin.StatusSuccess, 0, "", nil
	}
	return output, builtin.StatusSuccess, len(encoded), "", nil
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
