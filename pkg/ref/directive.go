package ref

import (
	"regexp"
	"sync"
)

type Directive struct {
	Name    string
	Handler func(ctx *Evaluator, node Node) (Node, error)
}

var (
	useDirectiveRegex = regexp.MustCompile(`^(?P<component>agent|tool|task)\((?P<scope>local|global)::(?P<path>.+)\)$`)
	refDirectiveRegex = regexp.MustCompile(`^(?P<scope>local|global)::(?P<path>.+)$`)
)

var (
	directives map[string]Directive
	once       sync.Once
)

func getDirectives() map[string]Directive {
	once.Do(func() {
		directives = map[string]Directive{
			"$use":   {Name: "$use", Handler: handleUse},
			"$ref":   {Name: "$ref", Handler: handleRef},
			"$merge": {Name: "$merge", Handler: handleMerge},
		}
	})
	return directives
}
