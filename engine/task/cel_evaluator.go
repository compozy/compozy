package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/google/cel-go/cel"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

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

// CELEvaluator implements ConditionEvaluator using CEL
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
		// Allow webhook filters to use `{ payload, headers, query: map[string]any }`
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
	// Apply options
	for _, opt := range opts {
		opt(evaluator)
	}
	// Create Ristretto cache with proper configuration
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

// normalizeExpression normalizes a CEL expression for better cache hit rate
func normalizeExpression(expression string) string {
	// Trim whitespace
	normalized := strings.TrimSpace(expression)
	// Replace multiple spaces with single space
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

// getProgram retrieves a compiled program from cache or compiles and caches it
func (c *CELEvaluator) getProgram(expression string) (cel.Program, error) {
	// Normalize expression for better cache hit rate
	cacheKey := normalizeExpression(expression)
	// Try to get from cache
	if program, found := c.programCache.Get(cacheKey); found {
		return program, nil
	}
	// Not in cache, compile it
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
	// Store in cache with cost of 1
	c.programCache.Set(cacheKey, program, 1)
	// Wait for the value to be processed
	c.programCache.Wait()
	return program, nil
}

// Evaluate executes CEL expression with resource limits
func (c *CELEvaluator) Evaluate(ctx context.Context, expression string, data map[string]any) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("context canceled before CEL evaluation: %w", err)
	}
	// Handle empty conditions as always true
	if strings.TrimSpace(expression) == "" {
		return true, nil
	}
	program, err := c.getProgram(expression)
	if err != nil {
		return false, err
	}
	out, details, err := program.ContextEval(ctx, data)
	if err != nil {
		return false, fmt.Errorf("CEL evaluation failed: %w", err)
	}
	// Log CEL evaluation cost for monitoring
	if details != nil {
		if cost := details.ActualCost(); cost != nil && c.costLimit > 0 {
			// Log cost metrics for monitoring and optimization
			costRatio := float64(*cost) / float64(c.costLimit)
			if costRatio > 0.8 {
				// Warn when approaching cost limit (80% threshold)
				log := logger.FromContext(ctx)
				log.Warn("CEL expression approaching cost limit",
					"cost", *cost,
					"limit", c.costLimit,
					"ratio_percent", costRatio*100)
			}
			// This should not happen as CEL would have errored, but keep for safety
			if *cost > c.costLimit {
				return false, core.NewError(
					fmt.Errorf("CEL expression exceeded cost limit: %d", *cost),
					"CEL_COST_EXCEEDED",
					map[string]any{"cost": *cost, "limit": c.costLimit},
				)
			}
		}
	}
	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression '%s' must return boolean, got %T", expression, out.Value())
	}
	return result, nil
}

// ValidateExpression validates a CEL expression without executing it
func (c *CELEvaluator) ValidateExpression(expression string) error {
	_, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("invalid CEL expression: %w", issues.Err())
	}
	return nil
}
