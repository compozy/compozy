package calltask

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

const toolID = "cp__call_task"

func init() { //nolint:gochecknoinits // builtin registration on package load
	native.RegisterProvider(Definition)
}

// Definition exposes the builtin definition for cp__call_task.
func Definition(env toolenv.Environment) builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:           toolID,
		Description:  "Execute a single task synchronously.",
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
		var output core.Output
		bytesWritten := 0
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
	if env == nil || env.TaskExecutor() == nil {
		return nil, status, 0, builtin.CodeInternal, builtin.Internal(errors.New("task executor unavailable"), nil)
	}
	nativeCfg := config.DefaultNativeToolsConfig()
	if appCfg := config.FromContext(ctx); appCfg != nil {
		nativeCfg = appCfg.Runtime.NativeTools
	}
	if !nativeCfg.CallTask.Enabled {
		return nil, status, 0, builtin.CodePermissionDenied, builtin.PermissionDenied(
			errors.New("call task tool disabled"),
			nil,
		)
	}
	input, code, err := decodeHandlerInput(payload)
	if err != nil {
		return nil, status, 0, code, err
	}
	req, code, err := buildTaskRequest(nativeCfg.CallTask, input)
	if err != nil {
		return nil, status, 0, code, err
	}
	exec := env.TaskExecutor()
	execStart := time.Now()
	result, execErr := exec.ExecuteTask(ctx, req)
	duration := time.Since(execStart)
	if execErr != nil {
		code, err := resolveExecutionError(execErr)
		return nil, status, 0, code, err
	}
	output := buildHandlerOutput(req, result, duration)
	encoded, err := json.Marshal(output)
	if err != nil {
		logger.FromContext(ctx).Warn("Failed to encode call_task output", "error", err)
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
