package calltasks

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/config"
	"github.com/mitchellh/mapstructure"
)

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

func buildTaskPlans(
	requests []TaskExecutionRequest,
	cfg config.NativeCallTasksConfig,
) ([]taskPlan, string, error) {
	if len(requests) == 0 {
		return nil, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("tasks array must include at least one entry"),
			map[string]any{errorKeyHint: "Add at least one task configuration to the \"tasks\" array."},
		)
	}
	plans := make([]taskPlan, len(requests))
	for i, req := range requests {
		normalized := normalizeExecutionRequest(req)
		if code, err := validateExecutionRequest(normalized); err != nil {
			return nil, code, err
		}
		reqBuilt, code, err := makeTaskRequest(normalized, cfg)
		if err != nil {
			return nil, code, err
		}
		plans[i] = taskPlan{index: i, request: reqBuilt, userConfig: normalized}
	}
	return plans, "", nil
}

func normalizeExecutionRequest(raw TaskExecutionRequest) TaskExecutionRequest {
	return TaskExecutionRequest{
		TaskID:    strings.TrimSpace(raw.TaskID),
		With:      raw.With,
		TimeoutMs: raw.TimeoutMs,
	}
}

func validateExecutionRequest(req TaskExecutionRequest) (string, error) {
	if req.TaskID == "" {
		return builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("task_id is required"),
			map[string]any{errorKeyHint: "Provide \"task_id\" for each entry in the \"tasks\" array."},
		)
	}
	if req.TimeoutMs < 0 {
		return builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("timeout_ms must be non-negative"),
			map[string]any{errorKeyHint: "Remove negative timeout or supply a positive integer (milliseconds)."},
		)
	}
	return "", nil
}

func makeTaskRequest(
	req TaskExecutionRequest,
	cfg config.NativeCallTasksConfig,
) (toolenv.TaskRequest, string, error) {
	timeout := cfg.DefaultTimeout
	if timeout < 0 {
		timeout = 0
	}
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}
	request := toolenv.TaskRequest{
		TaskID:  req.TaskID,
		Timeout: timeout,
	}
	if len(req.With) > 0 {
		copied, err := core.DeepCopy(req.With)
		if err != nil {
			return toolenv.TaskRequest{}, builtin.CodeInternal, builtin.Internal(
				fmt.Errorf("failed to copy with payload for task %s: %w", req.TaskID, err),
				nil,
			)
		}
		request.With = core.Input(copied)
	}
	return request, "", nil
}
