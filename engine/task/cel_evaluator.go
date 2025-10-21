package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/google/cel-go/cel"

	"github.com/compozy/compozy/pkg/logger"
)

// costWarningThreshold controls when to log that a CEL evaluation is approaching the configured cost limit.
const costWarningThreshold = 0.8

// CELEvaluatorOption configures CEL evaluator
type CELEvaluatorOption func(*CELEvaluator)

// WithCostLimit sets the cost limit for CEL evaluation
func WithCostLimit(limit uint64) CELEvaluatorOption {
	return func(e *CELEvaluator) {
		e.costLimit = limit
	}
}

// WithCacheSize sets the maximum cache size for compiled programs
func WithCacheSize(size int) CELEvaluatorOption {
	return func(e *CELEvaluator) {
		e.maxCacheSize = size
	}
}

// CELEvaluator provides CEL expression evaluation with caching and cost limits.
type CELEvaluator struct {
	env          *cel.Env
	costLimit    uint64
	maxCacheSize int
	programCache *ristretto.Cache[string, cel.Program]
}

// NewCELEvaluator creates a new CEL evaluator with security constraints
func NewCELEvaluator(opts ...CELEvaluatorOption) (*CELEvaluator, error) {
	env, err := cel.NewEnv(
		cel.Variable("signal", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("processor", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("task", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("workflow", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("tasks", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("input", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("with", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("env", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("item", cel.DynType),
		cel.Variable("index", cel.DynType),
		cel.Variable("parent", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("current", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("project", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("payload", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("headers", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("query", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}
	evaluator := &CELEvaluator{
		env:          env,
		costLimit:    1000, // Default cost limit
		maxCacheSize: 100,  // Default cache size
	}
	for _, opt := range opts {
		opt(evaluator)
	}
	cache, err := ristretto.NewCache(&ristretto.Config[string, cel.Program]{
		NumCounters: int64(evaluator.maxCacheSize * 10), // 10x the max items as recommended
		MaxCost:     int64(evaluator.maxCacheSize),      // Each program has cost of 1
		BufferItems: 64,                                 // Recommended buffer size
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create program cache: %w", err)
	}
	evaluator.programCache = cache
	return evaluator, nil
}

// getProgram retrieves a compiled program from cache or compiles and caches it
func (c *CELEvaluator) getProgram(expression string) (cel.Program, error) {
	if program, found := c.programCache.Get(expression); found {
		return program, nil
	}
	ast, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation failed: %w", issues.Err())
	}
	program, err := c.env.Program(ast,
		cel.EvalOptions(cel.OptExhaustiveEval, cel.OptTrackCost),
		cel.CostLimit(c.costLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL program: %w", err)
	}
	c.programCache.Set(expression, program, 1)
	c.programCache.Wait()
	return program, nil
}

// Evaluate executes CEL expression with resource limits
func (c *CELEvaluator) Evaluate(ctx context.Context, expression string, data map[string]any) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("context canceled before CEL evaluation: %w", err)
	}
	if strings.TrimSpace(expression) == "" {
		return true, nil
	}
	value, err := c.EvaluateValue(ctx, expression, data)
	if err != nil {
		return false, err
	}
	result, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression '%s' must return boolean, got %T", expression, value)
	}
	return result, nil
}

// EvaluateValue evaluates a CEL expression and returns the raw result.
func (c *CELEvaluator) EvaluateValue(
	ctx context.Context,
	expression string,
	data map[string]any,
) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled before CEL evaluation: %w", err)
	}
	program, err := c.getProgram(expression)
	if err != nil {
		return nil, err
	}
	out, details, err := program.ContextEval(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("CEL evaluation failed: %w", err)
	}
	if err := c.observeEvaluationCost(ctx, details); err != nil {
		return nil, err
	}
	return out.Value(), nil
}

func (c *CELEvaluator) observeEvaluationCost(ctx context.Context, details *cel.EvalDetails) error {
	if details == nil {
		return nil
	}
	if cost := details.ActualCost(); cost != nil && c.costLimit > 0 {
		costRatio := float64(*cost) / float64(c.costLimit)
		if costRatio > costWarningThreshold {
			logger.FromContext(ctx).Warn(
				"CEL expression approaching cost limit",
				"cost", *cost,
				"limit", c.costLimit,
				"ratio_percent", costRatio*100,
			)
		}
		if *cost > c.costLimit {
			return fmt.Errorf("CEL expression exceeded cost limit: %d (limit %d)", *cost, c.costLimit)
		}
	}
	return nil
}

// ValidateExpression validates a CEL expression without executing it
func (c *CELEvaluator) ValidateExpression(expression string) error {
	_, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("invalid CEL expression: %w", issues.Err())
	}
	return nil
}
