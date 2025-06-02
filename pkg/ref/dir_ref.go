package ref

import "fmt"

// -----------------------------------------------------------------------------
// $ref Directive
// -----------------------------------------------------------------------------

func validateRef(node Node) error {
	str, ok := node.(string)
	if !ok {
		return fmt.Errorf("$ref must be a string")
	}
	matches := refDirectiveRegex.FindStringSubmatch(str)
	if matches == nil {
		return fmt.Errorf("invalid $ref syntax: %s, expected format: <scope=local|global>::<gjson_path>[!merge:<options>]",
			str)
	}
	return nil
}

func handleRef(ctx EvaluatorContext, parentNode map[string]any, node Node) (Node, error) {
	str, ok := node.(string)
	if !ok {
		// This should never happen as validation passed
		return nil, fmt.Errorf("$ref must be a string")
	}
	matches := refDirectiveRegex.FindStringSubmatch(str)
	scope := matches[refIdxScope]
	gjsonPath := matches[refIdxPath]
	mergeOptsStr := ""
	if refIdxMergeOpts >= 0 && len(matches) > refIdxMergeOpts {
		mergeOptsStr = matches[refIdxMergeOpts]
	}

	result, err := ctx.ResolvePath(scope, gjsonPath)
	if err != nil {
		return nil, fmt.Errorf("$ref %s::%s: %w", scope, gjsonPath, err)
	}

	// Collect siblings
	siblings := make(map[string]any)
	for k, v := range parentNode {
		if k != "$ref" {
			siblings[k] = v
		}
	}

	// If no siblings, just return the result
	if len(siblings) == 0 {
		return result, nil
	}

	// Siblings exist - merge is enabled by default
	// Parse merge options if provided, otherwise use defaults
	mergeOpts := parseMergeOptions(mergeOptsStr)
	if mergeOptsStr == "" {
		// Enable merge with defaults when siblings exist
		mergeOpts.Enabled = true
	}

	// Perform inline merge based on result type
	return performInlineMerge(result, siblings, mergeOpts)
}

// performInlineMerge merges the directive result with sibling keys
func performInlineMerge(directiveResult Node, siblings map[string]any, opts MergeOptions) (Node, error) {
	// Handle replace strategy for inline merge - directive result wins
	if opts.Strategy == StrategyReplace {
		return directiveResult, nil
	}

	// Determine types
	switch result := directiveResult.(type) {
	case map[string]any:
		// Object merge - directive result is first source, siblings are second
		sources := []any{result, siblings}
		return mergeObjects(sources, opts.Strategy, opts.KeyConflict)
	case []any:
		// Array merge - convert siblings map to array if needed
		// This is an edge case - typically arrays wouldn't have sibling keys
		return nil, fmt.Errorf("cannot merge array result with object siblings")
	case nil:
		// Treat nil as empty object
		return siblings, nil
	default:
		// Scalar result
		return nil, fmt.Errorf("cannot merge scalar result with siblings: result is %T", directiveResult)
	}
}
