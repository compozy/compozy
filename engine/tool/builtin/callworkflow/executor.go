package callworkflow

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

const errorKeyHint = builtin.RemediationHintKey

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

func buildWorkflowRequest(
	cfg config.NativeCallWorkflowConfig,
	input handlerInput,
) (toolenv.WorkflowRequest, string, error) {
	var request toolenv.WorkflowRequest
	workflowID := strings.TrimSpace(input.WorkflowID)
	if workflowID == "" {
		return request, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			fmt.Errorf("workflow_id is required"),
			map[string]any{errorKeyHint: "Provide a valid \"workflow_id\" referencing a configured workflow."},
		)
	}
	if input.TimeoutMs < 0 {
		return request, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			fmt.Errorf("timeout_ms must be non-negative"),
			map[string]any{errorKeyHint: "Remove the negative timeout or supply a positive integer (milliseconds)."},
		)
	}
	timeout := cfg.DefaultTimeout
	if input.TimeoutMs > 0 {
		timeout = time.Duration(input.TimeoutMs) * time.Millisecond
	} else if timeout < 0 {
		return request, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			fmt.Errorf("call_workflow default timeout must be non-negative"),
			map[string]any{
				errorKeyHint: "Update runtime.native_tools.call_workflow.default_timeout " +
					"or supply a non-negative timeout_ms value.",
			},
		)
	}
	request = toolenv.WorkflowRequest{
		WorkflowID:    workflowID,
		InitialTaskID: strings.TrimSpace(input.InitialTaskID),
		Timeout:       timeout,
	}
	if len(input.Input) > 0 {
		copied, err := core.DeepCopy(input.Input)
		if err != nil {
			return toolenv.WorkflowRequest{}, builtin.CodeInternal, builtin.Internal(
				fmt.Errorf("failed to copy input payload: %w", err),
				nil,
			)
		}
		request.Input = core.Input(copied)
	}
	return request, "", nil
}

func buildHandlerOutput(
	req toolenv.WorkflowRequest,
	res *toolenv.WorkflowResult,
	duration time.Duration,
) core.Output {
	output := core.Output{
		"success":          true,
		"workflow_id":      req.WorkflowID,
		"workflow_exec_id": "",
		"status":           string(core.StatusSuccess),
		"duration_ms":      duration.Milliseconds(),
	}
	if res == nil {
		return output
	}
	output["workflow_exec_id"] = res.WorkflowExecID.String()
	status := res.Status
	if status == "" {
		status = string(core.StatusSuccess)
	}
	output["status"] = status
	if core.StatusType(status) != core.StatusSuccess {
		output["success"] = false
	}
	if res.Output != nil {
		if clone, err := res.Output.Clone(); err == nil && clone != nil {
			output["output"] = *clone
		} else if copied, err := core.DeepCopyOutputPtr(res.Output, (*core.Output)(nil)); err == nil && copied != nil {
			output["output"] = *copied
		} else {
			// last-resort shallow clone of map to avoid aliasing
			output["output"] = core.Output(core.CloneMap(map[string]any(*res.Output)))
		}
	}
	return output
}
