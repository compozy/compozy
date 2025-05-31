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

var directives = map[string]Directive{
	"$use": UseDirective,
	"$ref": RefDirective,
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
