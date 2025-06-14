package ref

import (
	"fmt"
	"maps"
	"regexp"
	"strings"
	"sync"
)

type Directive struct {
	Name      string
	Validator func(node Node) error
	Handler   func(ctx EvaluatorContext, parentNode map[string]any, node Node) (Node, error)
}

// MergeOptions holds the parsed inline merge options
type MergeOptions struct {
	Strategy    StrategyType
	KeyConflict KeyConflictType
}

var (
	// Updated regex patterns to capture optional inline merge syntax and resource scope
	useDirectiveRegex = regexp.MustCompile(
		`^(?P<component>agent|tool|task|mcp)\((?P<scope>local|global|resource)::(?P<path>.+?)\)(?:!merge:<(?P<merge_opts>[^>]*)>)?$`,
	)
	refDirectiveRegex = regexp.MustCompile(
		`^(?P<scope>local|global|resource)::(?P<path>.+?)(?:!merge:<(?P<merge_opts>[^>]*)>)?$`,
	)

	// Named group indices for safer extraction
	useIdxComponent = useDirectiveRegex.SubexpIndex("component")
	useIdxScope     = useDirectiveRegex.SubexpIndex("scope")
	useIdxPath      = useDirectiveRegex.SubexpIndex("path")
	useIdxMergeOpts = useDirectiveRegex.SubexpIndex("merge_opts")

	refIdxScope     = refDirectiveRegex.SubexpIndex("scope")
	refIdxPath      = refDirectiveRegex.SubexpIndex("path")
	refIdxMergeOpts = refDirectiveRegex.SubexpIndex("merge_opts")
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

// parseMergeOptions parses the inline merge options string
// Format: [strategy][,key_conflict]
// Examples: "deep", "shallow,error", "replace", ",first"
func parseMergeOptions(opts string) MergeOptions {
	result := MergeOptions{
		Strategy:    StrategyDeep,       // default for objects
		KeyConflict: KeyConflictReplace, // default is now replace
	}

	if opts == "" {
		return result
	}

	// Split by comma to get individual options
	parts := splitMergeOptions(opts)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if it's a strategy
		strategy := StrategyType(part)
		if strategy.IsValid() {
			result.Strategy = strategy
			continue
		}

		// Check if it's a key conflict option
		keyConflict := KeyConflictType(part)
		if keyConflict.IsValid() {
			result.KeyConflict = keyConflict
			continue
		}
	}

	return result
}

// splitMergeOptions splits the options string by comma, handling edge cases
func splitMergeOptions(opts string) []string {
	if opts == "" {
		return nil
	}
	return strings.Split(opts, ",")
}
