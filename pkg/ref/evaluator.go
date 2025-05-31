package ref

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

// Node represents a YAML node (map, slice, or scalar).
type Node any

// TransformUseFunc defines a callback for transforming $use results.
type TransformUseFunc func(component string, config Node) (key string, value Node, err error)

// EvalConfigOption configures EvalState.
type EvalConfigOption func(*Evaluator)

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

// WithTransformUse sets the $use transformation function.
func WithTransformUse(transform TransformUseFunc) EvalConfigOption {
	return func(ev *Evaluator) {
		ev.TransformUse = transform
	}
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
	return ev
}

// ResolvePath resolves a GJSON path in the given scope.
func (ev *Evaluator) ResolvePath(scope, path string) (Node, error) {
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
	return parseJSON(result.Raw)
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

// -----------------------------------------------------------------------------
// Evaluate
// -----------------------------------------------------------------------------

// Eval processes a node and resolves directives.
func (ev *Evaluator) Eval(node Node) (Node, error) {
	if node == nil {
		return nil, nil
	}

	if m, ok := node.(map[string]any); ok {
		// Check for directives in deterministic order
		for _, dirName := range []string{"$use", "$ref"} {
			if value, exists := m[dirName]; exists {
				// Stricter validation: directive nodes should only contain the directive key
				if len(m) != 1 {
					return nil, fmt.Errorf("%s node may not contain sibling keys", dirName)
				}
				if directive, ok := directives[dirName]; ok {
					result, err := directive.Handler(ev, value)
					if err != nil {
						return nil, err
					}
					// Recursively evaluate the result to resolve any nested directives
					return ev.Eval(result)
				}
			}
		}
		// Regular map processing
		result := make(map[string]any)
		for key, value := range m {
			evaluated, err := ev.Eval(value)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate %s: %w", key, err)
			}
			result[key] = evaluated
		}
		return result, nil
	}

	if s, ok := node.([]any); ok {
		result := make([]any, len(s))
		for i, value := range s {
			evaluated, err := ev.Eval(value)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate index %d: %w", i, err)
			}
			result[i] = evaluated
		}
		return result, nil
	}

	return node, nil
}
