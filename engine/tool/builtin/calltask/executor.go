package calltask

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/config"
	"github.com/mitchellh/mapstructure"
)

const errorKeyHint = "remediation_hint"

func decodeHandlerInput(payload map[string]any) (handlerInput, string, error) {
	var input handlerInput
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:      &input,
		TagName:     "mapstructure",
		ErrorUnused: true,
	})
	if err != nil {
		return input, builtin.CodeInternal, builtin.Internal(
			fmt.Errorf("failed to create decoder: %w", err),
			nil,
		)
	}
	if err := decoder.Decode(payload); err != nil {
		return input, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			fmt.Errorf("failed to decode input: %w", err),
			nil,
		)
	}
	return input, "", nil
}

func buildTaskRequest(cfg config.NativeCallTaskConfig, input handlerInput) (toolenv.TaskRequest, string, error) {
	var request toolenv.TaskRequest
	taskID := strings.TrimSpace(input.TaskID)
	if taskID == "" {
		return request, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			fmt.Errorf("task_id is required"),
			map[string]any{errorKeyHint: "Provide a valid \"task_id\" referencing a configured task."},
		)
	}
	if input.TimeoutMs < 0 {
		return request, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			fmt.Errorf("timeout_ms must be non-negative"),
			map[string]any{errorKeyHint: "Remove negative timeout or supply a positive integer (milliseconds)."},
		)
	}
	timeout := cfg.DefaultTimeout
	if timeout < 0 {
		timeout = 0
	}
	if input.TimeoutMs > 0 {
		timeout = time.Duration(input.TimeoutMs) * time.Millisecond
	}
	request = toolenv.TaskRequest{
		TaskID:  taskID,
		Timeout: timeout,
	}
	if len(input.With) > 0 {
		copied, err := core.DeepCopy(input.With)
		if err != nil {
			return toolenv.TaskRequest{}, builtin.CodeInternal, builtin.Internal(
				fmt.Errorf("failed to copy with payload: %w", err),
				nil,
			)
		}
		request.With = core.Input(copied)
	}
	return request, "", nil
}

func buildHandlerOutput(req toolenv.TaskRequest, res *toolenv.TaskResult, duration time.Duration) core.Output {
	output := core.Output{
		"success":     true,
		"task_id":     req.TaskID,
		"duration_ms": duration.Milliseconds(),
	}
	execID := ""
	if res != nil {
		execID = res.ExecID.String()
		if res.Output != nil {
			if clone, err := res.Output.Clone(); err == nil && clone != nil {
				output["output"] = *clone
			} else if res.Output != nil {
				output["output"] = *res.Output
			}
		}
	}
	output["exec_id"] = execID
	return output
}
