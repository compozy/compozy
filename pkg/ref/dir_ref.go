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

func handleRef(es *Evaluator, node Node) (Node, error) {
	str, ok := node.(string)
	if !ok {
		// This should never happen as validation passed
		return nil, fmt.Errorf("$ref must be a string")
	}
	matches := refDirectiveRegex.FindStringSubmatch(str)
	scope := matches[1]
	gjsonPath := matches[2]
	return es.ResolvePath(scope, gjsonPath)
}
