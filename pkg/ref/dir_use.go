package ref

import (
	"fmt"
)

// -----------------------------------------------------------------------------
// $use Directive
// -----------------------------------------------------------------------------

func validateUse(node Node) error {
	str, ok := node.(string)
	if !ok {
		return fmt.Errorf("$use must be a string")
	}
	matches := useDirectiveRegex.FindStringSubmatch(str)
	if matches == nil {
		return fmt.Errorf("invalid $use syntax: %s, expected format: "+
			"<component=agent|tool|task>(<scope=local|global>::<gjson_path>)[!merge:<options>]", str)
	}
	return nil
}

func handleUse(ctx EvaluatorContext, parentNode map[string]any, node Node) (Node, error) {
	str, ok := node.(string)
	if !ok {
		// This should never happen as validation passed
		return nil, fmt.Errorf("$use must be a string")
	}
	matches := useDirectiveRegex.FindStringSubmatch(str)
	component := matches[useIdxComponent]
	scope := matches[useIdxScope]
	gjsonPath := matches[useIdxPath]
	mergeOptsStr := ""
	if useIdxMergeOpts >= 0 && len(matches) > useIdxMergeOpts {
		mergeOptsStr = matches[useIdxMergeOpts]
	}

	// Resolve component configuration
	config, err := ctx.ResolvePath(scope, gjsonPath)
	if err != nil {
		return nil, fmt.Errorf("$use %s(%s::%s): %w", component, scope, gjsonPath, err)
	}

	result := make(map[string]any)
	transform := ctx.GetTransformUse()
	if transform != nil {
		key, value, err := transform(component, config)
		if err != nil {
			return nil, fmt.Errorf("failed to transform $use %s(%s::%s): %w", component, scope, gjsonPath, err)
		}
		result[key] = value
	} else {
		result[component] = config
	}

	// Collect siblings
	siblings := make(map[string]any)
	for k, v := range parentNode {
		if k != "$use" {
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

	// Perform inline merge - result is always an object from $use
	sources := []any{result, siblings}
	return mergeObjects(sources, mergeOpts.Strategy, mergeOpts.KeyConflict)
}
