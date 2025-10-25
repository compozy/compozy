package callworkflows

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

const (
	errorKeyHint       = "remediation_hint"
	errorKeyWorkflowID = "workflow_id"
)

type workflowPlan struct {
	index      int
	request    toolenv.WorkflowRequest
	userConfig WorkflowExecutionRequest
}

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

func buildWorkflowPlans(
	requests []WorkflowExecutionRequest,
	cfg config.NativeCallWorkflowsConfig,
) ([]workflowPlan, string, error) {
	if len(requests) == 0 {
		return nil, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("workflows array must include at least one entry"),
			map[string]any{errorKeyHint: "Add at least one workflow configuration to the \"workflows\" array."},
		)
	}
	plans := make([]workflowPlan, len(requests))
	for i, req := range requests {
		normalized := normalizeExecutionRequest(req)
		if code, err := validateExecutionRequest(normalized); err != nil {
			return nil, code, err
		}
		reqBuilt, code, err := makeWorkflowRequest(normalized, cfg)
		if err != nil {
			return nil, code, err
		}
		plans[i] = workflowPlan{index: i, request: reqBuilt, userConfig: normalized}
	}
	return plans, "", nil
}

func normalizeExecutionRequest(raw WorkflowExecutionRequest) WorkflowExecutionRequest {
	return WorkflowExecutionRequest{
		WorkflowID:    strings.TrimSpace(raw.WorkflowID),
		Input:         raw.Input,
		InitialTaskID: strings.TrimSpace(raw.InitialTaskID),
		TimeoutMs:     raw.TimeoutMs,
	}
}

func validateExecutionRequest(req WorkflowExecutionRequest) (string, error) {
	if req.WorkflowID == "" {
		return builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("workflow_id is required"),
			map[string]any{errorKeyHint: "Provide \"workflow_id\" for each entry in the \"workflows\" array."},
		)
	}
	if req.TimeoutMs < 0 {
		return builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("timeout_ms must be non-negative"),
			map[string]any{errorKeyHint: "Remove the negative timeout or supply a positive integer (milliseconds)."},
		)
	}
	return "", nil
}

func makeWorkflowRequest(
	req WorkflowExecutionRequest,
	cfg config.NativeCallWorkflowsConfig,
) (toolenv.WorkflowRequest, string, error) {
	timeout := cfg.DefaultTimeout
	if timeout < 0 {
		timeout = 0
	}
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}
	request := toolenv.WorkflowRequest{
		WorkflowID:    req.WorkflowID,
		InitialTaskID: req.InitialTaskID,
		Timeout:       timeout,
	}
	if len(req.Input) > 0 {
		copied, err := core.DeepCopy(req.Input)
		if err != nil {
			return toolenv.WorkflowRequest{}, builtin.CodeInternal, builtin.Internal(
				fmt.Errorf("failed to copy input payload for workflow %s: %w", req.WorkflowID, err),
				nil,
			)
		}
		request.Input = core.Input(copied)
	}
	return request, "", nil
}
