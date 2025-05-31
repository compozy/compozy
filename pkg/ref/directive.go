package ref

import (
	"fmt"
	"maps"
	"regexp"
	"sync"
)

type Directive struct {
	Name      string
	Validator func(node Node) error
	Handler   func(ctx EvaluatorContext, node Node) (Node, error)
}

var (
	useDirectiveRegex = regexp.MustCompile(`^(?P<component>agent|tool|task)\((?P<scope>local|global)::(?P<path>.+)\)$`)
	refDirectiveRegex = regexp.MustCompile(`^(?P<scope>local|global)::(?P<path>.+)$`)

	// Named group indices for safer extraction
	useIdxComponent = useDirectiveRegex.SubexpIndex("component")
	useIdxScope     = useDirectiveRegex.SubexpIndex("scope")
	useIdxPath      = useDirectiveRegex.SubexpIndex("path")

	refIdxScope = refDirectiveRegex.SubexpIndex("scope")
	refIdxPath  = refDirectiveRegex.SubexpIndex("path")
)

var (
	directives map[string]Directive
	once       sync.Once
	mu         sync.RWMutex // Protect directives map for concurrent access
)

func getDirectives() map[string]Directive {
	once.Do(func() {
		directives = map[string]Directive{
			"$use":   {Name: "$use", Validator: validateUse, Handler: handleUse},
			"$ref":   {Name: "$ref", Validator: validateRef, Handler: handleRef},
			"$merge": {Name: "$merge", Validator: validateMerge, Handler: handleMerge},
		}
	})
	mu.RLock()
	defer mu.RUnlock()
	// Return a copy to prevent external modification
	result := make(map[string]Directive)
	maps.Copy(result, directives)
	return result
}

// Register adds a new directive to the global registry.
// This must be called before any Evaluator is created.
// The directive name must start with '$'.
func Register(d Directive) error {
	if d.Name == "" {
		return fmt.Errorf("directive name cannot be empty")
	}
	if d.Name[0] != '$' {
		return fmt.Errorf("directive name must start with '$', got %q", d.Name)
	}
	if d.Handler == nil {
		return fmt.Errorf("directive handler cannot be nil")
	}

	// Ensure directives map is initialized
	once.Do(func() {
		directives = map[string]Directive{
			"$use":   {Name: "$use", Validator: validateUse, Handler: handleUse},
			"$ref":   {Name: "$ref", Validator: validateRef, Handler: handleRef},
			"$merge": {Name: "$merge", Validator: validateMerge, Handler: handleMerge},
		}
	})

	mu.Lock()
	defer mu.Unlock()

	if _, exists := directives[d.Name]; exists {
		return fmt.Errorf("directive %q already registered", d.Name)
	}

	directives[d.Name] = d
	return nil
}
