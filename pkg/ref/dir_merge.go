package ref

import (
	"fmt"
	"maps"

	"dario.cat/mergo"
)

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------

const (
	strategyDefault = "default"
	strategyDeep    = "deep"
	strategyShallow = "shallow"
	strategyConcat  = "concat"
	strategyPrepend = "prepend"
	strategyUnique  = "unique"

	keyConflictLast  = "last"
	keyConflictFirst = "first"
	keyConflictError = "error"
)

// -----------------------------------------------------------------------------
// $merge Directive
// -----------------------------------------------------------------------------

func handleMerge(ev *Evaluator, node Node) (Node, error) {
	// First check if the node itself is a directive that needs evaluation
	if evaluated, ok, err := tryEvaluateDirective(ev, node); ok {
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate directive in $merge: %w", err)
		}
		// Now process the evaluated result
		return handleMerge(ev, evaluated)
	}

	// Parse merge configuration
	sources, strategy, keyConflict, err := parseMergeConfig(node)
	if err != nil {
		return nil, err
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
	sourceType, err := determineMergeType(evaluatedSources)
	if err != nil {
		return nil, err
	}

	switch sourceType {
	case "object":
		return mergeObjects(evaluatedSources, strategy, keyConflict)
	case "array":
		return mergeArrays(evaluatedSources, strategy)
	default:
		// All sources are nil, return empty object
		return map[string]any{}, nil
	}
}

// tryEvaluateDirective checks if node is a directive that needs evaluation
func tryEvaluateDirective(ev *Evaluator, node Node) (Node, bool, error) {
	m, ok := node.(map[string]any)
	if !ok {
		return nil, false, nil
	}

	// Check if this is a directive node that needs evaluation first
	for _, dirName := range []string{"$use", "$ref", "$merge"} {
		if _, exists := m[dirName]; exists && len(m) == 1 {
			// This is a directive node, evaluate it first
			evaluated, err := ev.Eval(node)
			return evaluated, true, err
		}
	}
	return nil, false, nil
}

// parseMergeConfig extracts merge configuration from the node
func parseMergeConfig(node Node) (sources []any, strategy, keyConflict string, err error) {
	switch v := node.(type) {
	case []any:
		// Shorthand syntax
		return v, strategyDefault, keyConflictLast, nil
	case map[string]any:
		// Explicit syntax
		sourcesRaw, ok := v["sources"]
		if !ok {
			return nil, "", "", fmt.Errorf("$merge mapping must contain 'sources' key")
		}
		sourcesList, ok := sourcesRaw.([]any)
		if !ok {
			return nil, "", "", fmt.Errorf("$merge sources must be a sequence")
		}
		sources = sourcesList

		// Get strategy (default based on content type)
		if s, ok := v["strategy"].(string); ok {
			strategy = s
		} else {
			strategy = strategyDefault
		}

		// Get key_conflict option for objects (default: last)
		if kc, ok := v["key_conflict"].(string); ok {
			keyConflict = kc
		} else {
			keyConflict = keyConflictLast
		}

		// Validate no unknown keys
		for k := range v {
			if k != "sources" && k != "strategy" && k != "key_conflict" {
				return nil, "", "", fmt.Errorf("unknown key in $merge: %s", k)
			}
		}

		return sources, strategy, keyConflict, nil
	default:
		return nil, "", "", fmt.Errorf("$merge must be a sequence or mapping with 'sources'")
	}
}

// determineMergeType checks if all sources are objects, arrays, or nil
func determineMergeType(sources []any) (string, error) {
	var allMaps, allSlices bool
	var hasNonNil bool

	for _, src := range sources {
		if src == nil {
			continue
		}
		hasNonNil = true
		switch src.(type) {
		case map[string]any:
			if !allSlices {
				allMaps = true
			} else {
				return "", fmt.Errorf("$merge sources must be all objects or all arrays, not mixed")
			}
		case []any:
			if !allMaps {
				allSlices = true
			} else {
				return "", fmt.Errorf("$merge sources must be all objects or all arrays, not mixed")
			}
		default:
			return "", fmt.Errorf("$merge source must be an object or array, got %T", src)
		}
	}

	if !hasNonNil {
		return "nil", nil
	}
	if allMaps {
		return "object", nil
	}
	if allSlices {
		return "array", nil
	}
	return "", fmt.Errorf("$merge sources must be objects or arrays")
}

func mergeObjects(sources []any, strategy, keyConflict string) (Node, error) {
	if strategy == strategyDefault {
		strategy = strategyDeep
	}
	// Validate strategy
	if strategy != strategyDeep && strategy != strategyShallow {
		return nil, fmt.Errorf("invalid object merge strategy: %s (must be 'deep' or 'shallow')", strategy)
	}
	// Validate key_conflict
	if keyConflict != keyConflictLast && keyConflict != keyConflictFirst && keyConflict != keyConflictError {
		return nil, fmt.Errorf("invalid key_conflict: %s (must be 'last', 'first', or 'error')", keyConflict)
	}
	result := make(map[string]any)
	// For deep merge with "last" conflict resolution, we can use mergo
	if strategy == strategyDeep && keyConflict == keyConflictLast {
		return mergeObjectsWithMergo(sources)
	}
	// Custom logic for other cases
	for _, src := range sources {
		if src == nil {
			continue
		}
		srcMap, ok := src.(map[string]any)
		if !ok {
			continue // Skip non-maps (defensive)
		}

		if err := mergeObjectsCustom(result, srcMap, strategy, keyConflict); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// mergeObjectsWithMergo uses the mergo library for deep merge with last-wins
func mergeObjectsWithMergo(sources []any) (Node, error) {
	result := make(map[string]any)
	for _, src := range sources {
		if src == nil {
			continue
		}
		srcMap, ok := src.(map[string]any)
		if !ok {
			continue
		}
		if err := mergo.Merge(&result, srcMap, mergo.WithOverride); err != nil {
			return nil, fmt.Errorf("failed to merge maps: %w", err)
		}
	}
	return result, nil
}

// mergeObjectsCustom handles custom merge logic for specific strategies/conflicts
func mergeObjectsCustom(result, srcMap map[string]any, strategy, keyConflict string) error {
	for key, value := range srcMap {
		if existing, exists := result[key]; exists {
			switch keyConflict {
			case keyConflictError:
				return fmt.Errorf("key conflict: '%s' already exists", key)
			case keyConflictFirst:
				continue // Keep existing value
			case keyConflictLast:
				// Continue to merge or replace
			}

			if strategy == strategyDeep {
				// Deep merge if both values are maps
				if existingMap, ok1 := existing.(map[string]any); ok1 {
					if valueMap, ok2 := value.(map[string]any); ok2 {
						merged := make(map[string]any)
						maps.Copy(merged, existingMap)
						if err := mergo.Merge(&merged, valueMap, mergo.WithOverride); err != nil {
							return fmt.Errorf("failed to deep merge key '%s': %w", key, err)
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
	return nil
}

func mergeArrays(sources []any, strategy string) (Node, error) {
	if strategy == strategyDefault {
		strategy = strategyConcat
	}
	// Validate strategy
	if strategy != strategyConcat && strategy != strategyPrepend && strategy != strategyUnique {
		return nil, fmt.Errorf("invalid array merge strategy: %s (must be 'concat', 'prepend', or 'unique')", strategy)
	}
	switch strategy {
	case strategyConcat:
		return mergeArraysConcat(sources), nil
	case strategyPrepend:
		return mergeArraysPrepend(sources), nil
	case strategyUnique:
		return mergeArraysUnique(sources), nil
	default:
		// Should not reach here due to validation above
		return nil, fmt.Errorf("unexpected array merge strategy: %s", strategy)
	}
}

// mergeArraysConcat concatenates all arrays in order
func mergeArraysConcat(sources []any) []any {
	var result []any
	for _, src := range sources {
		if src == nil {
			continue
		}
		if srcSlice, ok := src.([]any); ok {
			result = append(result, srcSlice...)
		}
	}
	return result
}

// mergeArraysPrepend prepends each source array to the beginning
func mergeArraysPrepend(sources []any) []any {
	var result []any
	for _, src := range sources {
		if src == nil {
			continue
		}
		if srcSlice, ok := src.([]any); ok {
			// Prepend the current slice to the beginning of result
			result = append(srcSlice, result...)
		}
	}
	return result
}

// mergeArraysUnique concatenates arrays keeping only unique elements
func mergeArraysUnique(sources []any) []any {
	var result []any
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
	return result
}
