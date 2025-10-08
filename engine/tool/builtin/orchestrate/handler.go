package orchestrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	agentexec "github.com/compozy/compozy/engine/agent/exec"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/builtin/orchestrate/planner"
	"github.com/compozy/compozy/engine/tool/native"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/mitchellh/mapstructure"
)

const toolID = "cp__agent_orchestrate"

var (
	orchestrateInputSchema  = buildInputSchema()
	orchestrateOutputSchema = buildOutputSchema()
)

func init() { //nolint:gochecknoinits // required to register builtin definition with native catalog at startup
	native.RegisterProvider(Definition)
}

func Definition(env toolenv.Environment) builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:           toolID,
		Description:  "Compile and execute multi-agent plans expressed inline from the calling prompt.",
		InputSchema:  orchestrateInputSchema,
		OutputSchema: orchestrateOutputSchema,
		Handler:      newHandler(env),
	}
}

type handlerInput struct {
	Prompt         string         `mapstructure:"prompt"`
	Plan           map[string]any `mapstructure:"plan"`
	Bindings       map[string]any `mapstructure:"bindings"`
	MaxParallel    int            `mapstructure:"max_parallel"`
	TimeoutMs      int            `mapstructure:"timeout_ms"`
	DisablePlanner bool           `mapstructure:"disable_planner"`
}

func newHandler(env toolenv.Environment) builtin.Handler {
	return func(ctx context.Context, payload map[string]any) (core.Output, error) {
		start := time.Now()
		status := builtin.StatusFailure
		errorCode := ""
		bytes := 0
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
		log := logger.FromContext(ctx)
		if env == nil || env.AgentExecutor() == nil {
			err := errors.New("agent executor unavailable")
			errorCode = builtin.CodeInternal
			return nil, builtin.Internal(err, nil)
		}
		cfg := config.DefaultNativeToolsConfig()
		if appCfg := config.FromContext(ctx); appCfg != nil {
			cfg = appCfg.Runtime.NativeTools
		}
		if !cfg.AgentOrchestrator.Enabled {
			err := errors.New("agent orchestrator disabled")
			errorCode = builtin.CodePermissionDenied
			return nil, builtin.PermissionDenied(err, nil)
		}
		var input handlerInput
		if err := mapstructure.Decode(payload, &input); err != nil {
			errorCode = builtin.CodeInvalidArgument
			return nil, builtin.InvalidArgument(fmt.Errorf("failed to decode input: %w", err), nil)
		}
		plan, err := compileRequestPlan(ctx, cfg.AgentOrchestrator, input)
		if err != nil {
			var coreErr *core.Error
			if errors.As(err, &coreErr) {
				errorCode = coreErr.Code
			} else {
				errorCode = builtin.CodeInvalidArgument
			}
			return nil, err
		}
		limits := deriveLimits(cfg.AgentOrchestrator, input)
		engine := NewEngine(agentExecutorRunner{exec: env.AgentExecutor()}, limits)
		results, runErr := engine.Run(ctx, &plan)
		if runErr != nil {
			log.Warn("Agent orchestrator execution failed", "error", core.RedactError(runErr))
			errorCode = builtin.CodeInternal
		}
		output := renderOutput(results, plan.Bindings, runErr)
		if encoded, encodeErr := json.Marshal(output); encodeErr == nil {
			bytes = len(encoded)
		}
		if runErr == nil {
			status = builtin.StatusSuccess
		}
		return output, nil
	}
}

//nolint:gocyclo // validation branches reflect explicit PRD requirements
func compileRequestPlan(
	ctx context.Context,
	cfg config.NativeAgentOrchestratorConfig,
	input handlerInput,
) (Plan, error) {
	var zero Plan
	bindings := make(map[string]any)
	for k, v := range input.Bindings {
		bindings[k] = v
	}
	needsPlanner := len(input.Plan) == 0 && input.Prompt != ""
	client, hasClient := llmadapter.ClientFromContext(ctx)
	if needsPlanner && (cfg.Planner.Disabled || input.DisablePlanner) {
		err := errors.New("planner disabled for prompt input")
		return zero, builtin.InvalidArgument(err, map[string]any{"reason": "planner_disabled"})
	}
	if needsPlanner && !hasClient {
		err := errors.New("planner requires llm client in context")
		return zero, builtin.InvalidArgument(err, map[string]any{"field": "prompt"})
	}
	opts := planner.Options{
		Client:   client,
		MaxSteps: cfg.Planner.MaxSteps,
		Disabled: cfg.Planner.Disabled || input.DisablePlanner || !hasClient,
	}
	compiler, err := planner.NewCompiler(opts)
	if err != nil {
		return zero, builtin.Internal(err, nil)
	}
	compileInput := planner.CompileInput{
		Prompt:         input.Prompt,
		Plan:           input.Plan,
		Bindings:       bindings,
		DisablePlanner: cfg.Planner.Disabled || input.DisablePlanner,
	}
	plan, err := compiler.Compile(ctx, compileInput)
	if err != nil {
		switch {
		case errors.Is(err, planner.ErrPromptRequired):
			return zero, builtin.InvalidArgument(err, map[string]any{"field": "prompt"})
		case errors.Is(err, planner.ErrPlannerDisabled):
			return zero, builtin.InvalidArgument(err, map[string]any{"reason": "planner_disabled"})
		case errors.Is(err, planner.ErrPlannerRecursion):
			return zero, builtin.InvalidArgument(err, map[string]any{"reason": "planner_recursion"})
		case errors.Is(err, planner.ErrInvalidPlan):
			return zero, builtin.InvalidArgument(err, map[string]any{"field": "plan"})
		default:
			return zero, builtin.Internal(err, nil)
		}
	}
	return plan, nil
}

func deriveLimits(cfg config.NativeAgentOrchestratorConfig, input handlerInput) Limits {
	limits := Limits{
		MaxDepth:       cfg.MaxDepth,
		MaxSteps:       cfg.MaxSteps,
		MaxParallel:    cfg.MaxParallel,
		DefaultTimeout: cfg.DefaultTimeout,
	}
	if input.MaxParallel > 0 && (limits.MaxParallel == 0 || input.MaxParallel < limits.MaxParallel) {
		limits.MaxParallel = input.MaxParallel
	}
	if input.TimeoutMs > 0 {
		timeout := time.Duration(input.TimeoutMs) * time.Millisecond
		if limits.DefaultTimeout == 0 || timeout < limits.DefaultTimeout {
			limits.DefaultTimeout = timeout
		}
	}
	return limits
}

func renderOutput(results []StepResult, bindings map[string]any, runErr error) core.Output {
	stepObjs := make([]map[string]any, len(results))
	errorsOut := make([]map[string]any, 0)
	success := true
	for i := range results {
		stepObj, stepSuccess, stepErrors := convertStepResult(&results[i])
		if !stepSuccess {
			success = false
		}
		stepObjs[i] = stepObj
		if len(stepErrors) > 0 {
			errorsOut = append(errorsOut, stepErrors...)
		}
	}
	if runErr != nil {
		success = false
		errorsOut = append(errorsOut, map[string]any{"error": core.RedactError(runErr)})
	}
	output := core.Output{
		"success": success,
		"steps":   stepObjs,
	}
	if len(bindings) > 0 {
		output["bindings"] = cloneMap(bindings)
	}
	if len(errorsOut) > 0 {
		output["errors"] = errorsOut
	}
	return output
}

func convertStepResult(result *StepResult) (map[string]any, bool, []map[string]any) {
	if result == nil {
		return map[string]any{}, true, nil
	}
	obj := map[string]any{
		"id":     result.StepID,
		"type":   string(result.Type),
		"status": string(result.Status),
	}
	success := result.Status == StepStatusSuccess
	errorsOut := make([]map[string]any, 0)
	if !result.ExecID.IsZero() {
		obj["exec_id"] = result.ExecID.String()
	}
	if result.Output != nil {
		obj["output"] = cloneMap(*result.Output)
	}
	if result.Elapsed > 0 {
		obj["elapsed_ms"] = result.Elapsed.Milliseconds()
	}
	if result.Error != nil {
		errorsOut = append(errorsOut, map[string]any{"step_id": result.StepID, "error": core.RedactError(result.Error)})
	}
	if len(result.Children) > 0 {
		children := make([]map[string]any, len(result.Children))
		for idx := range result.Children {
			childObj, childSuccess, childErrors := convertStepResult(&result.Children[idx])
			if !childSuccess {
				success = false
			}
			children[idx] = childObj
			if len(childErrors) > 0 {
				errorsOut = append(errorsOut, childErrors...)
			}
		}
		obj["children"] = children
	}
	if result.Status != StepStatusSuccess {
		success = false
	}
	return obj, success, errorsOut
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

type agentExecutorRunner struct {
	exec toolenv.AgentExecutor
}

func (r agentExecutorRunner) Execute(
	ctx context.Context,
	req agentexec.ExecuteRequest,
) (*agentexec.ExecuteResult, error) {
	if r.exec == nil {
		return nil, errRunnerRequired
	}
	result, err := r.exec.ExecuteAgent(ctx, toolenv.AgentRequest{
		AgentID: req.AgentID,
		Action:  req.Action,
		Prompt:  req.Prompt,
		With:    req.With,
		Timeout: req.Timeout,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &agentexec.ExecuteResult{ExecID: result.ExecID, Output: result.Output}, nil
}

func buildInputSchema() *schema.Schema {
	planSchema, err := PlanSchema()
	if err != nil {
		return nil
	}
	return &schema.Schema{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "Natural language instructions describing the orchestration goal.",
			},
			"plan":            planSchema,
			"bindings":        map[string]any{"type": "object", "additionalProperties": map[string]any{}},
			"max_parallel":    map[string]any{"type": "integer", "minimum": 1},
			"timeout_ms":      map[string]any{"type": "integer", "minimum": 1},
			"disable_planner": map[string]any{"type": "boolean"},
		},
		"oneOf": []any{
			map[string]any{"required": []string{"plan"}},
			map[string]any{"required": []string{"prompt"}},
		},
		"additionalProperties": false,
	}
}

func buildOutputSchema() *schema.Schema {
	return &schema.Schema{
		"type":     "object",
		"required": []any{"success", "steps"},
		"properties": map[string]any{
			"success": map[string]any{"type": "boolean"},
			"steps": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"id", "type", "status"},
				},
			},
			"errors": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"step_id": map[string]any{"type": "string"},
						"error":   map[string]any{"type": "string"},
					},
				},
			},
			"bindings": map[string]any{
				"type": "object",
			},
		},
	}
}
