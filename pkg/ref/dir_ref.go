package ref

import "fmt"

// -----------------------------------------------------------------------------
// $ref Directive
// -----------------------------------------------------------------------------

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
