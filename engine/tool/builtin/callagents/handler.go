package callagents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/native"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/mitchellh/mapstructure"
)

const (
	toolID          = "cp__call_agents"
	stepTypeAgent   = "agent_execution"
	errorKeyHint    = "remediation_hint"
	errorKeyAgentID = "agent_id"
)

func init() { //nolint:gochecknoinits // builtin registration on startup
	native.RegisterProvider(Definition)
}

// Definition exposes the builtin definition for cp__call_agents.
func Definition(env toolenv.Environment) builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:           toolID,
		Description:  "Execute multiple agents in parallel.",
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
		var responseBytes int
		var output core.Output
		defer func() {
			builtin.RecordInvocation(
				ctx,
				toolID,
				builtin.RequestIDFromContext(ctx),
				status,
				time.Since(start),
				responseBytes,
				errorCode,
			)
		}()
		result, stat, bytesWritten, code, err := processRequest(ctx, env, payload, start)
		if err != nil {
			errorCode = code
			return nil, err
		}
		status = stat
		responseBytes = bytesWritten
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
	if env == nil || env.AgentExecutor() == nil {
		err := errors.New("agent executor unavailable")
		return nil, status, 0, builtin.CodeInternal, builtin.Internal(err, nil)
	}
	log := logger.FromContext(ctx)
	nativeCfg := config.DefaultNativeToolsConfig()
	if appCfg := config.FromContext(ctx); appCfg != nil {
		nativeCfg = appCfg.Runtime.NativeTools
	}
	agentsCfg := nativeCfg.CallAgents
	if !agentsCfg.Enabled {
		err := errors.New("call agents tool disabled")
		return nil, status, 0, builtin.CodePermissionDenied, builtin.PermissionDenied(err, nil)
	}
	input, code, err := decodeHandlerInput(payload)
	if err != nil {
		return nil, status, 0, code, err
	}
	plans, code, err := buildAgentPlans(input.Agents, agentsCfg)
	if err != nil {
		return nil, status, 0, code, err
	}
	log.Info(
		"Parallel agent execution requested",
		"agent_count", len(plans),
		"max_concurrent", agentsCfg.MaxConcurrent,
	)
	results := executeAgentsParallel(ctx, env, plans, agentsCfg, log)
	summary := summarizeResults(results, time.Since(start).Milliseconds())
	output := buildHandlerOutput(results, summary)
	log.Info(
		"Parallel agent execution complete",
		"total", summary.TotalCount,
		"success", summary.SuccessCount,
		"failed", summary.FailureCount,
		"duration_ms", summary.TotalDuration,
	)
	encoded, err := json.Marshal(output)
	if err != nil {
		log.Warn("Failed to encode call_agents output", "error", err)
		return output, builtin.StatusSuccess, 0, "", nil
	}
	return output, builtin.StatusSuccess, len(encoded), "", nil
}

func decodeHandlerInput(payload map[string]any) (handlerInput, string, error) {
	var input handlerInput
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:      &input,
		TagName:     "mapstructure",
		ErrorUnused: true,
	})
	if err != nil {
		return input, builtin.CodeInternal, builtin.Internal(fmt.Errorf("failed to create decoder: %w", err), nil)
	}
	if err := decoder.Decode(payload); err != nil {
		return input, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			fmt.Errorf("failed to decode input: %w", err),
			nil,
		)
	}
	return input, "", nil
}

func buildAgentPlans(
	requests []AgentExecutionRequest,
	cfg config.NativeCallAgentsConfig,
) ([]agentPlan, string, error) {
	if len(requests) == 0 {
		return nil, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("agents array must include at least one entry"),
			map[string]any{errorKeyHint: "Add at least one agent configuration to the \"agents\" array."},
		)
	}
	if cfg.MaxConcurrent > 0 && len(requests) > cfg.MaxConcurrent {
		return nil, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			fmt.Errorf("agents count %d exceeds max_concurrent limit %d", len(requests), cfg.MaxConcurrent),
			map[string]any{
				errorKeyHint: fmt.Sprintf(
					"Reduce the number of agents or update runtime.native_tools.call_agents.max_concurrent to at least %d.",
					len(requests),
				),
			},
		)
	}
	plans := make([]agentPlan, len(requests))
	for index, raw := range requests {
		plan, code, err := buildSinglePlan(raw, cfg, index)
		if err != nil {
			return nil, code, err
		}
		plans[index] = plan
	}
	return plans, "", nil
}

func buildSinglePlan(
	raw AgentExecutionRequest,
	cfg config.NativeCallAgentsConfig,
	index int,
) (agentPlan, string, error) {
	normalized := normalizeExecutionRequest(raw)
	if code, err := validateExecutionRequest(normalized); err != nil {
		return agentPlan{}, code, err
	}
	request, code, err := makeAgentRequest(normalized, cfg)
	if err != nil {
		return agentPlan{}, code, err
	}
	return agentPlan{
		index:      index,
		request:    request,
		userConfig: normalized,
	}, "", nil
}

func normalizeExecutionRequest(raw AgentExecutionRequest) AgentExecutionRequest {
	return AgentExecutionRequest{
		AgentID:   strings.TrimSpace(raw.AgentID),
		ActionID:  strings.TrimSpace(raw.ActionID),
		Prompt:    strings.TrimSpace(raw.Prompt),
		With:      raw.With,
		TimeoutMs: raw.TimeoutMs,
	}
}

func validateExecutionRequest(req AgentExecutionRequest) (string, error) {
	if req.AgentID == "" {
		return builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("agent_id is required"),
			map[string]any{
				errorKeyHint: "Call cp__list_agents to discover valid agent identifiers, then supply \"agent_id\".",
			},
		)
	}
	if req.ActionID == "" && req.Prompt == "" {
		return builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("either action_id or prompt must be provided"),
			map[string]any{
				errorKeyHint:    "Provide \"action_id\" for an agent action or a \"prompt\" for free-form execution.",
				errorKeyAgentID: req.AgentID,
			},
		)
	}
	if req.TimeoutMs < 0 {
		return builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("timeout_ms must be non-negative"),
			map[string]any{errorKeyHint: "Remove negative timeout or use a positive value in milliseconds."},
		)
	}
	return "", nil
}

func makeAgentRequest(
	req AgentExecutionRequest,
	cfg config.NativeCallAgentsConfig,
) (toolenv.AgentRequest, string, error) {
	timeout := max(cfg.DefaultTimeout, 0)
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}
	request := toolenv.AgentRequest{
		AgentID: req.AgentID,
		Action:  req.ActionID,
		Prompt:  req.Prompt,
		Timeout: timeout,
	}
	if len(req.With) == 0 {
		return request, "", nil
	}
	copiedWith, err := core.DeepCopy(req.With)
	if err != nil {
		return toolenv.AgentRequest{}, builtin.CodeInternal, builtin.Internal(
			fmt.Errorf("failed to copy with payload for agent %s: %w", req.AgentID, err),
			nil,
		)
	}
	request.With = core.Input(copiedWith)
	return request, "", nil
}

type executionSummary struct {
	TotalCount    int
	SuccessCount  int
	FailureCount  int
	TotalDuration int64
}

func summarizeResults(results []AgentExecutionResult, elapsedMs int64) executionSummary {
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

func buildHandlerOutput(results []AgentExecutionResult, summary executionSummary) core.Output {
	return core.Output{
		"results":           results,
		"total_count":       summary.TotalCount,
		"success_count":     summary.SuccessCount,
		"failure_count":     summary.FailureCount,
		"total_duration_ms": summary.TotalDuration,
	}
}
