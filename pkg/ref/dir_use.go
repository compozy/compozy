package ref

import "fmt"

// -----------------------------------------------------------------------------
// $use Directive
// -----------------------------------------------------------------------------

func handleUse(es *Evaluator, node Node) (Node, error) {
	str, ok := node.(string)
	if !ok {
		return nil, fmt.Errorf("$use must be a string")
	}

	matches := useDirectiveRegex.FindStringSubmatch(str)
	if matches == nil {
		return nil, fmt.Errorf("invalid $use syntax: %s, expected format: "+
			"<component=agent|tool|task>(<scope=local|global>::<gjson_path>)", str)
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
