package ref

import (
	"fmt"
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

type (
	StrategyType    string
	KeyConflictType string
)

const (
	StrategyDefault StrategyType = "default"
	StrategyDeep    StrategyType = "deep"
	StrategyShallow StrategyType = "shallow"
	StrategyReplace StrategyType = "replace"
	StrategyConcat  StrategyType = "concat"
	StrategyPrepend StrategyType = "prepend"
	StrategyUnique  StrategyType = "unique"
	StrategyAppend  StrategyType = "append" // alias for concat
	StrategyUnion   StrategyType = "union"  // alias for unique

	KeyConflictReplace KeyConflictType = "replace"
	KeyConflictFirst   KeyConflictType = "first"
	KeyConflictError   KeyConflictType = "error"
)

func (s StrategyType) String() string {
	return string(s)
}

func (s StrategyType) IsValid() bool {
	return s == StrategyDefault ||
		s == StrategyDeep ||
		s == StrategyShallow ||
		s == StrategyReplace ||
		s == StrategyConcat ||
		s == StrategyPrepend ||
		s == StrategyUnique ||
		s == StrategyAppend ||
		s == StrategyUnion
}

func (s StrategyType) isValidForObjects() bool {
	return s == StrategyDeep || s == StrategyShallow || s == StrategyReplace
}

func (s StrategyType) isValidForArrays() bool {
	return s == StrategyConcat || s == StrategyPrepend || s == StrategyUnique ||
		s == StrategyAppend || s == StrategyUnion || s == StrategyReplace
}

func (k KeyConflictType) String() string {
	return string(k)
}

func (k KeyConflictType) IsValid() bool {
	return k == KeyConflictReplace ||
		k == KeyConflictFirst ||
		k == KeyConflictError
}

// -----------------------------------------------------------------------------
// $merge Directive
// -----------------------------------------------------------------------------

func validateMerge(node Node) error {
	// Check if this is potentially a directive node that needs evaluation first
	if isDirectiveNode(node) {
		return nil
	}

	switch v := node.(type) {
	case []any:
		return validateMergeArray(v)
	case map[string]any:
		return validateMergeMap(v)
	default:
		return fmt.Errorf("$merge must be a sequence or mapping with 'sources'")
	}
}

func isDirectiveNode(node Node) bool {
	m, ok := node.(map[string]any)
	if !ok || len(m) != 1 {
		return false
	}
	for key := range m {
		if key == "$use" || key == "$ref" || key == "$merge" {
			return true
		}
	}
	return false
}

func validateMergeArray(v []any) error {
	if len(v) == 0 {
		return fmt.Errorf("$merge sources cannot be empty")
	}
	return nil
}

func validateMergeMap(v map[string]any) error {
	// Must have 'sources' key
	sourcesRaw, ok := v["sources"]
	if !ok {
		return fmt.Errorf("$merge mapping must contain 'sources' key")
	}
	sourcesList, ok := sourcesRaw.([]any)
	if !ok {
		return fmt.Errorf("$merge sources must be a sequence")
	}
	if len(sourcesList) == 0 {
		return fmt.Errorf("$merge sources cannot be empty")
	}

	// Validate no unknown keys
	for k := range v {
		if k != "sources" && k != "strategy" && k != "key_conflict" {
			return fmt.Errorf("unknown key in $merge: %s", k)
		}
	}
	return nil
}

func handleMerge(ctx EvaluatorContext, parentNode map[string]any, node Node) (Node, error) {
	// Check for sibling keys - $merge doesn't support inline merge
	if len(parentNode) > 1 {
		return nil, fmt.Errorf("$merge directive cannot have sibling keys")
	}

	// First check if the node itself is a directive that needs evaluation
	if evaluated, ok, err := tryEvaluateDirective(ctx, node); ok {
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate directive in $merge: %w", err)
		}
		// Now process the evaluated result
		return handleMerge(ctx, parentNode, evaluated)
	}

	// Parse merge configuration (validation already done)
	sources, strategy, keyConflict := parseMergeConfig(node)

	// Evaluate all sources first
	evaluatedSources := make([]any, len(sources))
	for i, src := range sources {
		evaluated, err := ctx.Eval(src)
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
func tryEvaluateDirective(ctx EvaluatorContext, node Node) (Node, bool, error) {
	m, ok := node.(map[string]any)
	if !ok {
		return nil, false, nil
	}

	// Check if this is a directive node that needs evaluation first
	for _, dirName := range []string{"$use", "$ref", "$merge"} {
		if _, exists := m[dirName]; exists && len(m) == 1 {
			// This is a directive node, evaluate it first
			evaluated, err := ctx.Eval(node)
			return evaluated, true, err
		}
	}
	return nil, false, nil
}

// parseMergeConfig extracts merge configuration from the node (assumes validation passed)
func parseMergeConfig(node Node) (sources []any, strategy StrategyType, keyConflict KeyConflictType) {
	switch v := node.(type) {
	case []any:
		// Shorthand syntax
		return v, StrategyDefault, KeyConflictReplace
	case map[string]any:
		// Explicit syntax
		sourcesRaw, ok := v["sources"]
		if !ok {
			// Should never happen after validation
			panic("parseMergeConfig: sources key not found")
		}
		sources, ok = sourcesRaw.([]any)
		if !ok {
			// Should never happen after validation
			panic("parseMergeConfig: sources is not an array")
		}

		// Get strategy (default based on content type)
		if s, ok := v["strategy"].(string); ok {
			strategy = StrategyType(s)
		} else {
			strategy = StrategyDefault
		}

		// Get key_conflict option for objects (default: replace)
		if kc, ok := v["key_conflict"].(string); ok {
			keyConflict = KeyConflictType(kc)
		} else {
			keyConflict = KeyConflictReplace
		}

		return sources, strategy, keyConflict
	default:
		// Should never reach here after validation
		panic("unexpected node type in parseMergeConfig")
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

func mergeObjects(sources []any, strategy StrategyType, keyConflict KeyConflictType) (Node, error) {
	if strategy == StrategyDefault {
		strategy = StrategyDeep
	}

	// Handle replace strategy - just return the last non-nil source
	if strategy == StrategyReplace {
		for i := len(sources) - 1; i >= 0; i-- {
			if sources[i] != nil {
				return sources[i], nil
			}
		}
		return map[string]any{}, nil
	}

	// Validate strategy with specific error message
	if !strategy.isValidForObjects() {
		return nil, fmt.Errorf("invalid object merge strategy: %s (must be 'deep', 'shallow', or 'replace')", strategy)
	}

	// Validate key_conflict with specific error message
	if !keyConflict.IsValid() {
		return nil, fmt.Errorf("invalid key_conflict: %s (must be 'replace', 'first', or 'error')", keyConflict)
	}

	// For the common case (deep + replace), use optimized implementation
	if strategy == StrategyDeep && keyConflict == KeyConflictReplace {
		return deepMergeMaps(sources), nil
	}

	// Custom logic for other cases
	result := make(map[string]any)
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

// deepMergeMaps performs an optimized deep merge with last-wins semantics
func deepMergeMaps(sources []any) map[string]any {
	if len(sources) == 0 {
		return make(map[string]any)
	}
	// Start with the first non-nil map
	var result map[string]any
	startIdx := 0
	for i, src := range sources {
		if src != nil {
			if srcMap, ok := src.(map[string]any); ok {
				result = make(map[string]any, len(srcMap))
				for k, v := range srcMap {
					result[k] = v
				}
				startIdx = i + 1
				break
			}
		}
	}
	if result == nil {
		return make(map[string]any)
	}
	// Merge remaining sources
	for i := startIdx; i < len(sources); i++ {
		src := sources[i]
		if src == nil {
			continue
		}
		srcMap, ok := src.(map[string]any)
		if !ok {
			continue
		}
		for key, srcValue := range srcMap {
			if existingValue, exists := result[key]; exists {
				// Both are maps - merge recursively
				if existingMap, ok1 := existingValue.(map[string]any); ok1 {
					if srcValueMap, ok2 := srcValue.(map[string]any); ok2 {
						deepMergeInPlace(existingMap, srcValueMap)
						continue
					}
				}
			}
			// Otherwise, just replace
			result[key] = srcValue
		}
	}

	return result
}

// deepMergeInPlace merges src into dst, modifying dst in place
func deepMergeInPlace(dst, src map[string]any) {
	for key, srcValue := range src {
		if dstValue, exists := dst[key]; exists {
			// Both are maps - merge recursively
			if dstMap, ok1 := dstValue.(map[string]any); ok1 {
				if srcMap, ok2 := srcValue.(map[string]any); ok2 {
					deepMergeInPlace(dstMap, srcMap)
					continue
				}
			}
		}
		// Otherwise, just replace
		dst[key] = srcValue
	}
}

// mergeObjectsCustom handles custom merge logic for specific strategies/conflicts
func mergeObjectsCustom(result, srcMap map[string]any, strategy StrategyType, keyConflict KeyConflictType) error {
	for key, value := range srcMap {
		if existing, exists := result[key]; exists {
			switch keyConflict {
			case KeyConflictError:
				return fmt.Errorf("key conflict: '%s' already exists", key)
			case KeyConflictFirst:
				continue // Keep existing value
			case KeyConflictReplace:
				// Continue to merge or replace
			}

			// Only need to handle deep merge here
			if strategy == StrategyDeep {
				// Deep merge if both values are maps
				if existingMap, ok1 := existing.(map[string]any); ok1 {
					if valueMap, ok2 := value.(map[string]any); ok2 {
						// Instead of creating a new map and copying, merge in-place
						if err := mergeObjectsCustom(existingMap, valueMap, strategy, keyConflict); err != nil {
							return fmt.Errorf("failed to deep merge key '%s': %w", key, err)
						}
						continue
					}
				}
			}
		}
		// Shallow merge, non-map value, or new key - just assign directly
		result[key] = value
	}
	return nil
}

func mergeArrays(sources []any, strategy StrategyType) (Node, error) {
	if strategy == StrategyDefault {
		strategy = StrategyConcat
	}

	// Handle aliases
	switch strategy {
	case StrategyAppend:
		strategy = StrategyConcat
	case StrategyUnion:
		strategy = StrategyUnique
	}

	// Handle replace strategy - just return the last non-nil source
	if strategy == StrategyReplace {
		for i := len(sources) - 1; i >= 0; i-- {
			if sources[i] != nil {
				return sources[i], nil
			}
		}
		return []any{}, nil
	}

	// Validate strategy with specific error message
	if !strategy.isValidForArrays() {
		return nil, fmt.Errorf("invalid array merge strategy: %s (must be 'concat', 'prepend', 'unique', or 'replace')",
			strategy)
	}

	switch strategy {
	case StrategyConcat:
		return mergeArraysConcat(sources), nil
	case StrategyPrepend:
		return mergeArraysPrepend(sources), nil
	case StrategyUnique:
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
