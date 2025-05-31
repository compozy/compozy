package ref

import "fmt"

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
			"<component=agent|tool|task>(<scope=local|global>::<gjson_path>)", str)
	}
	return nil
}

func handleUse(ctx EvaluatorContext, node Node) (Node, error) {
	str, ok := node.(string)
	if !ok {
		// This should never happen as validation passed
		return nil, fmt.Errorf("$use must be a string")
	}
	matches := useDirectiveRegex.FindStringSubmatch(str)
	component := matches[useIdxComponent]
	scope := matches[useIdxScope]
	gjsonPath := matches[useIdxPath]

	// Resolve component configuration
	config, err := ctx.ResolvePath(scope, gjsonPath)
	if err != nil {
		return nil, fmt.Errorf("$use %s(%s::%s): %w", component, scope, gjsonPath, err)
	}

	// Apply transformation
	transform := ctx.GetTransformUse()
	if transform != nil {
		key, value, err := transform(component, config)
		if err != nil {
			return nil, fmt.Errorf("failed to transform $use %s(%s::%s): %w", component, scope, gjsonPath, err)
		}
		return map[string]any{key: value}, nil
	}

	// Default: return map with component as key
	return map[string]any{component: config}, nil
}
