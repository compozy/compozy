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
		return fmt.Errorf("invalid $ref syntax: %s, expected format: <scope=local|global>::<gjson_path>", str)
	}
	return nil
}

func handleRef(ctx EvaluatorContext, node Node) (Node, error) {
	str, ok := node.(string)
	if !ok {
		// This should never happen as validation passed
		return nil, fmt.Errorf("$ref must be a string")
	}
	matches := refDirectiveRegex.FindStringSubmatch(str)
	scope := matches[refIdxScope]
	gjsonPath := matches[refIdxPath]

	result, err := ctx.ResolvePath(scope, gjsonPath)
	if err != nil {
		return nil, fmt.Errorf("$ref %s::%s: %w", scope, gjsonPath, err)
	}
	return result, nil
}
