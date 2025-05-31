package ref

import (
	"fmt"
	"regexp"
)

type Directive struct {
	Name    string
	Handler func(ctx *Evaluator, node Node) (Node, error)
}

var (
	useDirectiveRegex = regexp.MustCompile(`^(?P<component>agent|tool|task)\((?P<scope>local|global)::(?P<path>.+)\)$`)
	refDirectiveRegex = regexp.MustCompile(`^(?P<scope>local|global)::(?P<path>.+)$`)
)

var directives map[string]Directive

func init() {
	directives = map[string]Directive{
		"$use":   UseDirective,
		"$ref":   RefDirective,
		"$merge": MergeDirective,
	}
}

// -----------------------------------------------------------------------------
// $use Directive
// -----------------------------------------------------------------------------

var UseDirective = Directive{
	Name: "$use", Handler: handleUse,
}

func handleUse(es *Evaluator, node Node) (Node, error) {
	str, ok := node.(string)
	if !ok {
		return nil, fmt.Errorf("$use must be a string")
	}

	matches := useDirectiveRegex.FindStringSubmatch(str)
	if matches == nil {
		return nil, fmt.Errorf("invalid $use syntax: %s, expected format: <component=agent|tool|task>(<scope=local|global>::<gjson_path>)", str)
	}

	component := matches[1]
	scope := matches[2]
	gjsonPath := matches[3]

	// Resolve component configuration
	config, err := es.ResolvePath(scope, gjsonPath)
	if err != nil {
		return nil, err
	}

	// Apply transformation
	if es.TransformUse == nil {
		// Default: return map with component as key
		return map[string]any{component: config}, nil
	}
	key, value, err := es.TransformUse(component, config)
	if err != nil {
		return nil, fmt.Errorf("failed to transform $use: %w", err)
	}
	return map[string]any{key: value}, nil
}

// -----------------------------------------------------------------------------
// $ref Directive
// -----------------------------------------------------------------------------

var RefDirective = Directive{
	Name: "$ref", Handler: handleRef,
}

func handleRef(es *Evaluator, node Node) (Node, error) {
	str, ok := node.(string)
	if !ok {
		return nil, fmt.Errorf("$ref must be a string")
	}
	matches := refDirectiveRegex.FindStringSubmatch(str)
	if matches == nil {
		return nil, fmt.Errorf("invalid $ref syntax: %s, expected format: <scope=local|global>::<gjson_path>", str)
	}
	scope := matches[1]
	gjsonPath := matches[2]
	return es.ResolvePath(scope, gjsonPath)
}

// -----------------------------------------------------------------------------
// $merge Directive
// -----------------------------------------------------------------------------

var MergeDirective = Directive{
	Name: "$merge", Handler: handleMerge,
}

func handleMerge(ev *Evaluator, node Node) (Node, error) {
	// The node can be either:
	// 1. A sequence (shorthand) - implicitly { strategy: default, sources: <sequence> }
	// 2. A mapping containing a 'sources' key plus optional merge options
	// 3. A directive (like $ref) that should be evaluated first
	var sources []any
	var strategy string
	var keyConflict string

	// First check if the node itself is a map with a directive
	if m, ok := node.(map[string]any); ok {
		// Check if this is a directive node that needs evaluation first
		for _, dirName := range []string{"$use", "$ref", "$merge"} {
			if _, exists := m[dirName]; exists && len(m) == 1 {
				// This is a directive node, evaluate it first
				evaluated, err := ev.Eval(node)
				if err != nil {
					return nil, fmt.Errorf("failed to evaluate directive in $merge: %w", err)
				}
				// Now process the evaluated result
				return handleMerge(ev, evaluated)
			}
		}
	}

	switch v := node.(type) {
	case []any:
		// Shorthand syntax
		sources = v
		strategy = "default"
		keyConflict = "last"
	case map[string]any:
		// Explicit syntax
		sourcesRaw, ok := v["sources"]
		if !ok {
			return nil, fmt.Errorf("$merge mapping must contain 'sources' key")
		}
		sourcesList, ok := sourcesRaw.([]any)
		if !ok {
			return nil, fmt.Errorf("$merge sources must be a sequence")
		}
		sources = sourcesList

		// Get strategy (default based on content type)
		if s, ok := v["strategy"].(string); ok {
			strategy = s
		} else {
			strategy = "default"
		}

		// Get key_conflict option for objects (default: last)
		if kc, ok := v["key_conflict"].(string); ok {
			keyConflict = kc
		} else {
			keyConflict = "last"
		}

		// Validate no unknown keys
		for k := range v {
			if k != "sources" && k != "strategy" && k != "key_conflict" {
				return nil, fmt.Errorf("unknown key in $merge: %s", k)
			}
		}
	default:
		return nil, fmt.Errorf("$merge must be a sequence or mapping with 'sources'")
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("$merge sources cannot be empty")
	}

	// Evaluate all sources first
	evaluatedSources := make([]any, len(sources))
	for i, src := range sources {
		evaluated, err := ev.Eval(src)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate $merge source at index %d: %w", i, err)
		}
		evaluatedSources[i] = evaluated
	}

	// Determine if we're merging objects or arrays
	var allMaps, allSlices bool
	var hasNonNil bool
	for _, src := range evaluatedSources {
		if src == nil {
			continue
		}
		hasNonNil = true
		switch src.(type) {
		case map[string]any:
			if !allSlices {
				allMaps = true
			} else {
				return nil, fmt.Errorf("$merge sources must be all objects or all arrays, not mixed")
			}
		case []any:
			if !allMaps {
				allSlices = true
			} else {
				return nil, fmt.Errorf("$merge sources must be all objects or all arrays, not mixed")
			}
		default:
			return nil, fmt.Errorf("$merge source must be an object or array, got %T", src)
		}
	}

	if !hasNonNil {
		// All sources are nil, return empty object
		return map[string]any{}, nil
	}

	if allMaps {
		return mergeObjects(evaluatedSources, strategy, keyConflict)
	} else if allSlices {
		return mergeArrays(evaluatedSources, strategy)
	}

	return nil, fmt.Errorf("$merge sources must be objects or arrays")
}

func mergeObjects(sources []any, strategy, keyConflict string) (Node, error) {
	if strategy == "default" {
		strategy = "deep"
	}

	// Validate strategy
	if strategy != "deep" && strategy != "shallow" {
		return nil, fmt.Errorf("invalid object merge strategy: %s (must be 'deep' or 'shallow')", strategy)
	}

	// Validate key_conflict
	if keyConflict != "last" && keyConflict != "first" && keyConflict != "error" {
		return nil, fmt.Errorf("invalid key_conflict: %s (must be 'last', 'first', or 'error')", keyConflict)
	}

	result := make(map[string]any)

	for _, src := range sources {
		if src == nil {
			continue
		}
		srcMap, ok := src.(map[string]any)
		if !ok {
			continue // Skip non-maps (defensive)
		}

		for key, value := range srcMap {
			if existing, exists := result[key]; exists {
				switch keyConflict {
				case "error":
					return nil, fmt.Errorf("key conflict: '%s' already exists", key)
				case "first":
					continue // Keep existing value
				case "last":
					// Continue to merge or replace
				}

				if strategy == "deep" {
					// Deep merge if both values are maps
					if existingMap, ok1 := existing.(map[string]any); ok1 {
						if valueMap, ok2 := value.(map[string]any); ok2 {
							merged, err := mergeObjects([]any{existingMap, valueMap}, "deep", keyConflict)
							if err != nil {
								return nil, err
							}
							result[key] = merged
							continue
						}
					}
				}
			}
			// Shallow merge or non-map value
			result[key] = value
		}
	}

	return result, nil
}

func mergeArrays(sources []any, strategy string) (Node, error) {
	if strategy == "default" {
		strategy = "concat"
	}

	// Validate strategy
	if strategy != "concat" && strategy != "prepend" && strategy != "unique" {
		return nil, fmt.Errorf("invalid array merge strategy: %s (must be 'concat', 'prepend', or 'unique')", strategy)
	}

	var result []any

	switch strategy {
	case "concat":
		for _, src := range sources {
			if src == nil {
				continue
			}
			if srcSlice, ok := src.([]any); ok {
				result = append(result, srcSlice...)
			}
		}
	case "prepend":
		// Process sources in normal order, but prepend each to the beginning
		for _, src := range sources {
			if src == nil {
				continue
			}
			if srcSlice, ok := src.([]any); ok {
				// Prepend the current slice to the beginning of result
				result = append(srcSlice, result...)
			}
		}
	case "unique":
		seen := make(map[string]bool)
		for _, src := range sources {
			if src == nil {
				continue
			}
			if srcSlice, ok := src.([]any); ok {
				for _, item := range srcSlice {
					// Create a unique key for the item
					key := fmt.Sprintf("%v", item)
					if !seen[key] {
						seen[key] = true
						result = append(result, item)
					}
				}
			}
		}
	}

	return result, nil
}
