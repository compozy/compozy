package callagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/native"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/mitchellh/mapstructure"
)

const (
	toolID = "cp__call_agent"
)

func init() { //nolint:gochecknoinits // register builtin on startup
	native.RegisterProvider(Definition)
}

// Definition exposes the builtin definition for cp__call_agent.
func Definition(env toolenv.Environment) builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:           toolID,
		Description:  "Execute a single agent action or prompt by identifier.",
		InputSchema:  &inputSchema,
		OutputSchema: &outputSchema,
		Handler:      newHandler(env),
	}
}

type handlerInput struct {
	AgentID   string         `json:"agent_id"   mapstructure:"agent_id"`
	ActionID  string         `json:"action_id"  mapstructure:"action_id"`
	Prompt    string         `json:"prompt"     mapstructure:"prompt"`
	With      map[string]any `json:"with"       mapstructure:"with"`
	TimeoutMs int            `json:"timeout_ms" mapstructure:"timeout_ms"`
}

func newHandler(env toolenv.Environment) builtin.Handler {
	return func(ctx context.Context, payload map[string]any) (core.Output, error) {
		start := time.Now()
		status := builtin.StatusFailure
		errorCode := ""
		var bytes int
		var output core.Output
		defer func() {
			builtin.RecordInvocation(
				ctx,
				toolID,
				builtin.RequestIDFromContext(ctx),
				status,
				time.Since(start),
				bytes,
				errorCode,
			)
		}()
		result, stat, written, code, err := processRequest(ctx, env, payload)
		if err != nil {
			errorCode = code
			return nil, err
		}
		status = stat
		bytes = written
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
	if env == nil || env.AgentExecutor() == nil {
		err := errors.New("agent executor unavailable")
		return nil, status, 0, builtin.CodeInternal, builtin.Internal(err, nil)
	}
	cfg := config.DefaultNativeToolsConfig()
	if appCfg := config.FromContext(ctx); appCfg != nil {
		cfg = appCfg.Runtime.NativeTools
	}
	if !cfg.CallAgent.Enabled {
		err := errors.New("call agent tool disabled")
		return nil, status, 0, builtin.CodePermissionDenied, builtin.PermissionDenied(err, nil)
	}
	input, code, err := decodeHandlerInput(payload)
	if err != nil {
		return nil, status, 0, code, err
	}
	req, code, err := buildAgentRequest(cfg.CallAgent, input)
	if err != nil {
		return nil, status, 0, code, err
	}
	executor := env.AgentExecutor()
	res, execErr := executor.ExecuteAgent(ctx, req)
	if execErr != nil {
		var cerr *core.Error
		if errors.As(execErr, &cerr) {
			return nil, status, 0, cerr.Code, cerr
		}
		return nil, status, 0, builtin.CodeInternal, builtin.Internal(execErr, nil)
	}
	output := buildOutput(req, res)
	encoded, err := json.Marshal(output)
	if err != nil {
		logger.FromContext(ctx).Warn("Failed to encode call_agent output", "error", err)
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

func buildAgentRequest(cfg config.NativeCallAgentConfig, input handlerInput) (toolenv.AgentRequest, string, error) {
	agentID := strings.TrimSpace(input.AgentID)
	if agentID == "" {
		return toolenv.AgentRequest{}, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("agent_id is required"),
			map[string]any{
				"remediation_hint": "Call cp__list_agents to discover valid agent identifiers, then provide \"agent_id\".",
			},
		)
	}
	actionID := strings.TrimSpace(input.ActionID)
	prompt := strings.TrimSpace(input.Prompt)
	if actionID == "" && prompt == "" {
		return toolenv.AgentRequest{}, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			errors.New("either action_id or prompt must be provided"),
			map[string]any{
				"remediation_hint": "Provide either \"action_id\" to run a specific action or " +
					"a \"prompt\" for the agent to interpret.",
			},
		)
	}
	if input.TimeoutMs < 0 {
		return toolenv.AgentRequest{}, builtin.CodeInvalidArgument, builtin.InvalidArgument(
			fmt.Errorf("timeout_ms must be non-negative"),
			map[string]any{"remediation_hint": "Remove negative timeout or supply a positive integer (milliseconds)."},
		)
	}
	timeout := max(cfg.DefaultTimeout, 0)
	if input.TimeoutMs > 0 {
		timeout = time.Duration(input.TimeoutMs) * time.Millisecond
	}
	req := toolenv.AgentRequest{
		AgentID: agentID,
		Action:  actionID,
		Prompt:  prompt,
		Timeout: timeout,
	}
	if len(input.With) > 0 {
		copiedWith, err := core.DeepCopy(input.With)
		if err != nil {
			return toolenv.AgentRequest{}, builtin.CodeInternal, builtin.Internal(
				fmt.Errorf("failed to copy with parameter: %w", err),
				nil,
			)
		}
		req.With = core.Input(copiedWith)
	}
	return req, "", nil
}

func buildOutput(req toolenv.AgentRequest, res *toolenv.AgentResult) core.Output {
	output := core.Output{
		"success":  true,
		"agent_id": req.AgentID,
	}
	if req.Action != "" {
		output["action_id"] = req.Action
	}
	if res != nil {
		if !res.ExecID.IsZero() {
			output["exec_id"] = res.ExecID.String()
		}
		if res.Output != nil {
			if clone, err := res.Output.Clone(); err == nil && clone != nil {
				output["response"] = *clone
			} else {
				output["response"] = *res.Output
			}
		}
	}
	return output
}

//nolint:unused // retained for future schema generation helpers
func buildInputSchema() *schema.Schema {
	return &schema.Schema{
		"type": "object",
		"properties": map[string]any{
			"agent_id": map[string]any{
				"type":        "string",
				"description": "Identifier of the agent to execute. Call cp__list_agents first to discover available IDs.",
			},
			"action_id": map[string]any{
				"type":        "string",
				"description": "Optional action identifier for the agent. Required when the agent defines multiple actions.",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "Optional natural language instructions passed to the agent when executing prompt-driven flows.",
			},
			"with": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{},
				"description":          "Structured action input payload matching the agent action schema.",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"description": "Optional timeout override in milliseconds for the agent execution.",
			},
		},
		"required": []any{"agent_id"},
		"oneOf": []any{
			map[string]any{"required": []string{"action_id"}},
			map[string]any{"required": []string{"prompt"}},
		},
		"additionalProperties": false,
	}
}

//nolint:unused // retained for future schema generation helpers
func buildOutputSchema() *schema.Schema {
	return &schema.Schema{
		"type":     "object",
		"required": []any{"success", "agent_id"},
		"properties": map[string]any{
			"success":   map[string]any{"type": "boolean"},
			"agent_id":  map[string]any{"type": "string"},
			"action_id": map[string]any{"type": "string"},
			"exec_id":   map[string]any{"type": "string"},
			"response": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{},
			},
		},
		"additionalProperties": true,
	}
}
