package planner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/schema"
	specpkg "github.com/compozy/compozy/engine/tool/builtin/orchestrate/spec"
	toolcontext "github.com/compozy/compozy/engine/tool/context"
	"github.com/compozy/compozy/pkg/logger"
)

var (
	ErrPlannerDisabled  = errors.New("planner disabled")
	ErrPromptRequired   = errors.New("planner requires prompt or plan")
	ErrPlannerRecursion = errors.New("planner recursion detected")
	ErrInvalidPlan      = errors.New("planner produced invalid plan")
)

const (
	defaultMaxSteps   = 12
	plannerSchemaName = "cp__agent_orchestrate.plan"
)

const plannerSystemPrompt = "" +
	"You are Compozy's orchestration planner.\n" +
	"Convert a user's natural language request into a JSON execution plan that follows the provided schema.\n" +
	"- Always return a single JSON object with no surrounding text.\n" +
	"- Use sequential step identifiers (step_1, step_2, step_3, ...).\n" +
	"- Set every step status to \"pending\".\n" +
	"- For agent steps, include agent_id and optional action_id, prompt, with, result_key.\n" +
	"- For parallel steps, include child steps array and optional merge strategy."

type Options struct {
	Client   llmadapter.LLMClient
	MaxSteps int
	Disabled bool
	Schema   *schema.Schema
}

type Compiler struct {
	client   llmadapter.LLMClient
	schema   *schema.Schema
	disabled bool
	maxSteps int
}

type CompileInput struct {
	Prompt         string
	Plan           map[string]any
	Bindings       map[string]any
	DisablePlanner bool
}

func NewCompiler(opts Options) (*Compiler, error) {
	if opts.Client == nil && !opts.Disabled {
		return nil, errors.New("planner requires llm client")
	}
	schemaRef := opts.Schema
	if schemaRef == nil {
		planSchema, err := specpkg.PlanSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to load plan schema: %w", err)
		}
		schemaRef = planSchema
	}
	maxSteps := opts.MaxSteps
	if maxSteps <= 0 {
		maxSteps = defaultMaxSteps
	}
	return &Compiler{
		client:   opts.Client,
		schema:   schemaRef,
		disabled: opts.Disabled,
		maxSteps: maxSteps,
	}, nil
}

func (c *Compiler) Compile(ctx context.Context, input CompileInput) (specpkg.Plan, error) {
	var zero specpkg.Plan
	if len(input.Plan) > 0 {
		return c.compileStructured(input)
	}
	trimmedPrompt := strings.TrimSpace(input.Prompt)
	if trimmedPrompt == "" {
		return zero, ErrPromptRequired
	}
	if c.disabled || input.DisablePlanner {
		return zero, ErrPlannerDisabled
	}
	if toolcontext.PlannerToolsDisabled(ctx) {
		return zero, ErrPlannerRecursion
	}
	return c.compilePrompt(ctx, trimmedPrompt, input.Bindings)
}

func (c *Compiler) compileStructured(input CompileInput) (specpkg.Plan, error) {
	var zero specpkg.Plan
	if input.Plan == nil {
		return zero, ErrPromptRequired
	}
	plan, err := specpkg.DecodePlanMap(input.Plan)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrInvalidPlan, err)
	}
	if err := c.finalizePlan(&plan, input.Bindings); err != nil {
		return zero, err
	}
	return plan, nil
}

func (c *Compiler) compilePrompt(
	ctx context.Context,
	prompt string,
	bindings map[string]any,
) (specpkg.Plan, error) {
	var zero specpkg.Plan
	log := logger.FromContext(ctx)
	reqSchema, err := c.schema.Clone()
	if err != nil {
		return zero, fmt.Errorf("failed to clone plan schema: %w", err)
	}
	userPrompt := buildUserPrompt(prompt, bindings)
	request := &llmadapter.LLMRequest{
		SystemPrompt: plannerSystemPrompt,
		Messages: []llmadapter.Message{
			{Role: llmadapter.RoleUser, Content: userPrompt},
		},
		Options: llmadapter.CallOptions{
			Temperature: 0,
			ToolChoice:  "none",
			OutputFormat: llmadapter.NewJSONSchemaOutputFormat(
				plannerSchemaName,
				reqSchema,
				true,
			),
			ForceJSON: true,
		},
	}
	callCtx := toolcontext.DisablePlannerTools(ctx)
	response, err := c.client.GenerateContent(callCtx, request)
	if err != nil {
		return zero, fmt.Errorf("planner llm call failed: %w", err)
	}
	planMap, err := decodePlanJSON(response)
	if err != nil {
		log.Warn("Planner returned invalid payload", "error", err)
		return zero, err
	}
	plan, err := specpkg.DecodePlanMap(planMap)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrInvalidPlan, err)
	}
	if err := c.finalizePlan(&plan, bindings); err != nil {
		return zero, err
	}
	log.Debug("Planner compiled plan", "steps", len(plan.Steps), "plan_id", plan.ID)
	return plan, nil
}

func (c *Compiler) finalizePlan(plan *specpkg.Plan, bindings map[string]any) error {
	if plan == nil {
		return fmt.Errorf("%w: missing plan", ErrInvalidPlan)
	}
	if plan.ID == "" {
		plan.ID = "agent_orchestrate_plan"
	}
	for idx := range plan.Steps {
		step := &plan.Steps[idx]
		if step.ID == "" {
			step.ID = fmt.Sprintf("step_%02d", idx+1)
		}
		step.Status = specpkg.StepStatusPending
	}
	if err := c.mergeBindings(plan, bindings); err != nil {
		return err
	}
	return c.enforceMaxSteps(plan)
}

func (c *Compiler) mergeBindings(plan *specpkg.Plan, extra map[string]any) error {
	if plan == nil {
		return fmt.Errorf("%w: missing plan", ErrInvalidPlan)
	}
	if len(plan.Bindings) == 0 && len(extra) == 0 {
		return nil
	}
	var result map[string]any
	if len(plan.Bindings) > 0 {
		cloned, err := core.DeepCopy(plan.Bindings)
		if err != nil {
			return fmt.Errorf("failed to copy plan bindings: %w", err)
		}
		result = cloned
	}
	if len(extra) > 0 {
		clonedExtra, err := core.DeepCopy(extra)
		if err != nil {
			return fmt.Errorf("failed to copy request bindings: %w", err)
		}
		if result == nil {
			result = clonedExtra
		} else {
			for key, value := range clonedExtra {
				result[key] = value
			}
		}
	}
	plan.Bindings = result
	return nil
}

func (c *Compiler) enforceMaxSteps(plan *specpkg.Plan) error {
	if plan == nil {
		return fmt.Errorf("%w: missing plan", ErrInvalidPlan)
	}
	if c.maxSteps > 0 && len(plan.Steps) > c.maxSteps {
		return fmt.Errorf("%w: plan has %d steps, exceeds %d", ErrInvalidPlan, len(plan.Steps), c.maxSteps)
	}
	return nil
}

func decodePlanJSON(resp *llmadapter.LLMResponse) (map[string]any, error) {
	if resp == nil {
		return nil, fmt.Errorf("%w: empty response", ErrInvalidPlan)
	}
	raw := strings.TrimSpace(resp.Content)
	if raw == "" {
		return nil, fmt.Errorf("%w: empty response", ErrInvalidPlan)
	}
	var plan map[string]any
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPlan, err)
	}
	return plan, nil
}

func buildUserPrompt(prompt string, bindings map[string]any) string {
	var b strings.Builder
	b.WriteString("Natural language request:\n")
	b.WriteString(strings.TrimSpace(prompt))
	if len(bindings) > 0 {
		if data, err := json.Marshal(bindings); err == nil {
			b.WriteString("\n\nExisting bindings (JSON):\n")
			b.Write(data)
		}
	}
	b.WriteString("\n\nRespond ONLY with valid JSON for the plan.")
	b.WriteString(" Use sequential step IDs like step_1 and set every step status to \"pending\".")
	return b.String()
}
