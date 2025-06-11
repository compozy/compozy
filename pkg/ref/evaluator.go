package ref

import (
	"encoding/json"
	"fmt"

	"github.com/dgraph-io/ristretto"
	"github.com/tidwall/gjson"
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

// Node represents a YAML node (map, slice, or scalar).
type Node any

// TransformUseFunc defines a callback for transforming $use results.
type TransformUseFunc func(component string, config Node) (key string, value Node, err error)

// PreEvalFunc defines a callback for preprocessing nodes before evaluation.
type PreEvalFunc func(node Node) (Node, error)

// EvalConfigOption configures EvalState.
type EvalConfigOption func(*Evaluator)

// EvaluatorContext provides the evaluation context methods used by directive handlers
type EvaluatorContext interface {
	ResolvePath(scope, path string) (Node, error)
	Eval(node Node) (Node, error)
	GetTransformUse() TransformUseFunc
}

// WithLocalScope sets the local scope.
func WithLocalScope(scope map[string]any) EvalConfigOption {
	return func(ev *Evaluator) {
		ev.LocalScope = scope
	}
}

// WithGlobalScope sets the global scope.
func WithGlobalScope(scope map[string]any) EvalConfigOption {
	return func(ev *Evaluator) {
		ev.GlobalScope = scope
	}
}

// WithScopes sets both local and global scopes.
func WithScopes(local, global map[string]any) EvalConfigOption {
	return func(ev *Evaluator) {
		ev.LocalScope = local
		ev.GlobalScope = global
	}
}

// WithTransformUse sets the $use transformation function.
func WithTransformUse(transform TransformUseFunc) EvalConfigOption {
	return func(ev *Evaluator) {
		ev.TransformUse = transform
	}
}

// WithPreEval sets the pre-evaluation hook that is called on every node before evaluation.
func WithPreEval(hook PreEvalFunc) EvalConfigOption {
	return func(ev *Evaluator) {
		ev.PreEval = hook
	}
}

// CacheConfig holds cache configuration options
type CacheConfig struct {
	MaxCost     int64 // Maximum cost of cache (approximately memory in bytes)
	NumCounters int64 // Number of counters for tracking frequency
	BufferItems int64 // Number of keys per Get buffer
}

// DefaultCacheConfig returns sensible defaults for the cache
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxCost:     100 << 20, // 100 MB
		NumCounters: 1e7,       // 10 million
		BufferItems: 64,
	}
}

// WithCache enables caching with the given configuration
func WithCache(config CacheConfig) EvalConfigOption {
	return func(ev *Evaluator) {
		ev.cacheConfig = &config
	}
}

// WithCacheEnabled enables caching with default configuration
func WithCacheEnabled() EvalConfigOption {
	config := DefaultCacheConfig()
	return WithCache(config)
}

// -----------------------------------------------------------------------------
// EvalConfig
// -----------------------------------------------------------------------------

// Evaluator holds evaluation state.
// Once created, it's safe to share across goroutines as it becomes read-only.
type Evaluator struct {
	LocalScope   map[string]any
	GlobalScope  map[string]any
	localJSON    []byte // Cached JSON representation of LocalScope
	globalJSON   []byte // Cached JSON representation of GlobalScope
	Directives   map[string]Directive
	TransformUse TransformUseFunc
	PreEval      PreEvalFunc
	cache        *ristretto.Cache[string, Node] // Path resolution cache
	cacheConfig  *CacheConfig
}

// NewEvaluator creates a new evaluation state with the given options.
func NewEvaluator(options ...EvalConfigOption) *Evaluator {
	ev := &Evaluator{}
	for _, opt := range options {
		opt(ev)
	}
	// Cache JSON representations to avoid re-encoding on every lookup
	if ev.LocalScope != nil {
		ev.localJSON = mustJSON(ev.LocalScope)
	}
	if ev.GlobalScope != nil {
		ev.globalJSON = mustJSON(ev.GlobalScope)
	}
	// Initialize cache if configured
	if ev.cacheConfig != nil {
		cache, err := ristretto.NewCache(&ristretto.Config[string, Node]{
			NumCounters: ev.cacheConfig.NumCounters,
			MaxCost:     ev.cacheConfig.MaxCost,
			BufferItems: ev.cacheConfig.BufferItems,
			Cost: func(value Node) int64 {
				// Estimate cost based on the serialized size
				if data, err := json.Marshal(value); err == nil {
					return int64(len(data))
				}
				// Default cost if marshaling fails
				return 100
			},
		})
		if err == nil {
			ev.cache = cache
		}
		// Silently ignore cache initialization errors - evaluator works without cache
	}
	return ev
}

// ResolvePath resolves a GJSON path in the given scope.
func (ev *Evaluator) ResolvePath(scope, path string) (Node, error) {
	// Check cache first if available
	cacheKey := scope + "::" + path
	if ev.cache != nil {
		if value, found := ev.cache.Get(cacheKey); found {
			return value, nil
		}
	}

	var dataJSON []byte
	switch scope {
	case "local":
		if ev.localJSON == nil {
			return nil, fmt.Errorf("local scope is not configured")
		}
		dataJSON = ev.localJSON
	case "global":
		if ev.globalJSON == nil {
			return nil, fmt.Errorf("global scope is not configured")
		}
		dataJSON = ev.globalJSON
	default:
		return nil, fmt.Errorf("invalid scope: %s", scope)
	}

	result := gjson.GetBytes(dataJSON, path)
	if !result.Exists() {
		return nil, fmt.Errorf("path %s not found in %s scope", path, scope)
	}
	node, err := parseJSON(result.Raw)
	if err != nil {
		return nil, err
	}

	// Store in cache if available
	if ev.cache != nil {
		ev.cache.Set(cacheKey, node, 0) // Cost will be calculated by the Cost function
		ev.cache.Wait()                 // Ensure the item is processed
	}

	return node, nil
}

func mustJSON(data any) []byte {
	bytes, err := json.Marshal(data)
	if err != nil {
		panic(fmt.Sprintf("failed to serialize scope: %v", err))
	}
	return bytes
}

func parseJSON(raw string) (Node, error) {
	var node any
	if err := json.Unmarshal([]byte(raw), &node); err != nil {
		return nil, err
	}
	return node, nil
}

// GetTransformUse returns the TransformUse function
func (ev *Evaluator) GetTransformUse() TransformUseFunc {
	return ev.TransformUse
}

// WithLocalScope sets the local scope.
func (ev *Evaluator) WithLocalScope(scope map[string]any) *Evaluator {
	if ev.LocalScope == nil {
		ev.LocalScope = make(map[string]any)
	}
	ev.LocalScope = scope
	ev.localJSON = mustJSON(scope)
	return ev
}

// WithGlobalScope sets the global scope.
func (ev *Evaluator) WithGlobalScope(scope map[string]any) *Evaluator {
	if ev.GlobalScope == nil {
		ev.GlobalScope = make(map[string]any)
	}
	ev.GlobalScope = scope
	ev.globalJSON = mustJSON(scope)
	return ev
}

// -----------------------------------------------------------------------------
// Evaluate
// -----------------------------------------------------------------------------

// Eval processes a node and resolves directives.
func (ev *Evaluator) Eval(node Node) (Node, error) {
	// Start with a fresh seen map for cycle detection
	return ev.eval(node, make(map[string]struct{}))
}

// eval is the internal recursive evaluation method that carries cycle detection state
func (ev *Evaluator) eval(node Node, seen map[string]struct{}) (Node, error) {
	if node == nil {
		return nil, nil
	}

	// Apply pre-evaluation hook if configured
	if ev.PreEval != nil {
		preprocessed, err := ev.PreEval(node)
		if err != nil {
			return nil, fmt.Errorf("pre-evaluation hook failed: %w", err)
		}
		node = preprocessed
	}

	switch v := node.(type) {
	case map[string]any:
		return ev.evalMap(v, seen)
	case []any:
		return ev.evalSlice(v, seen)
	default:
		return node, nil
	}
}

// evalMap processes a map node, checking for directives and recursively evaluating values
func (ev *Evaluator) evalMap(parent map[string]any, seen map[string]struct{}) (Node, error) {
	// Check for directives
	allDirectives := getDirectives()
	directiveCount := 0

	for dirName := range allDirectives {
		if _, exists := parent[dirName]; exists {
			directiveCount++
		}
	}
	if directiveCount > 1 {
		return nil, fmt.Errorf("multiple directives are not allowed in a map")
	}

	for dirName, directive := range allDirectives {
		if value, exists := parent[dirName]; exists {
			// Run validator first if present
			if directive.Validator != nil {
				if err := directive.Validator(value); err != nil {
					return nil, err
				}
			}
			// Pass seen map to handler through a wrapper evaluator
			wrapperEv := &evaluatorWithSeen{Evaluator: ev, seen: seen}
			result, err := directive.Handler(wrapperEv, parent, value)
			if err != nil {
				return nil, err
			}
			// Recursively evaluate the result to resolve any nested directives
			return ev.eval(result, seen)
		}
	}
	// Regular map processing
	result := make(map[string]any)
	for key, value := range parent {
		evaluated, err := ev.eval(value, seen)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate %s: %w", key, err)
		}
		result[key] = evaluated
	}
	return result, nil
}

// evalSlice processes a slice node, recursively evaluating each element
func (ev *Evaluator) evalSlice(s []any, seen map[string]struct{}) (Node, error) {
	result := make([]any, len(s))
	for i, value := range s {
		evaluated, err := ev.eval(value, seen)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate index %d: %w", i, err)
		}
		result[i] = evaluated
	}
	return result, nil
}

// evaluatorWithSeen wraps an Evaluator with cycle detection state
type evaluatorWithSeen struct {
	*Evaluator
	seen map[string]struct{}
}

// ResolvePath overrides the base method to add cycle detection
func (ev *evaluatorWithSeen) ResolvePath(scope, path string) (Node, error) {
	key := scope + "::" + path
	if _, exists := ev.seen[key]; exists {
		return nil, fmt.Errorf("cyclic reference detected at %s", key)
	}
	ev.seen[key] = struct{}{}
	defer delete(ev.seen, key)

	// Get the raw node from the scope
	node, err := ev.Evaluator.ResolvePath(scope, path)
	if err != nil {
		return nil, err
	}

	// Evaluate the resolved node to handle any nested directives
	return ev.Eval(node)
}

// Eval delegates to the internal eval method with the current seen map
func (ev *evaluatorWithSeen) Eval(node Node) (Node, error) {
	return ev.eval(node, ev.seen)
}

// GetTransformUse returns the TransformUse function
func (ev *evaluatorWithSeen) GetTransformUse() TransformUseFunc {
	return ev.TransformUse
}
