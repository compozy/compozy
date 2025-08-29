package autoload

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/tidwall/gjson"
)

// ResourceResolver implements ref.ResourceResolver for AutoLoader
type ResourceResolver struct {
	registry *ConfigRegistry
}

// NewResourceResolver creates a new resource resolver
func NewResourceResolver(registry *ConfigRegistry) *ResourceResolver {
	return &ResourceResolver{
		registry: registry,
	}
}

// ResolveResource resolves resource:: references by querying the ConfigRegistry
// Supports selectors like:
//   - #(id=='name')        - Find by ID
//   - #(id=='name').field  - Find by ID and access field
//   - field                - Direct field access (for single resources)
func (r *ResourceResolver) ResolveResource(resourceType, selector string) (ref.Node, error) {
	// Parse the selector to extract ID and field path
	id, fieldPath, err := ParseResourceSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("invalid resource selector '%s': %w", selector, err)
	}

	// Get the configuration from registry
	config, err := r.registry.Get(resourceType, id)
	if err != nil {
		return nil, err
	}

	// If no field path, return a defensive copy to prevent mutations
	if fieldPath == "" {
		return createDefensiveCopy(config), nil
	}

	// Apply field path using GJSON
	return ApplyFieldPath(config, fieldPath)
}

// createDefensiveCopy creates a defensive copy to prevent data races
func createDefensiveCopy(node ref.Node) ref.Node {
	switch v := node.(type) {
	case map[string]any:
		// Create a shallow copy for maps to prevent shared state mutations
		result := make(map[string]any, len(v))
		maps.Copy(result, v)
		return result
	case []any:
		// Create a copy for slices
		result := make([]any, len(v))
		copy(result, v)
		return result
	default:
		// Primitives are safe to return as-is
		return node
	}
}

// ParseResourceSelector parses resource selectors
// Supports:
//   - #(id=='name')        -> id="name", fieldPath=""
//   - #(id=='name').field  -> id="name", fieldPath="field"
//   - name                 -> id="name", fieldPath=""
//   - name.field           -> id="name", fieldPath="field"
func ParseResourceSelector(selector string) (id string, fieldPath string, err error) {
	if selector == "" {
		return "", "", fmt.Errorf("selector cannot be empty")
	}

	// Handle explicit ID selector format: #(id=='value').field
	if strings.HasPrefix(selector, "#(id==") {
		idPattern := regexp.MustCompile(`^#\(id==['"]([^'"]*)['"]\)(.*)$`)
		matches := idPattern.FindStringSubmatch(selector)
		if matches == nil {
			return "", "", fmt.Errorf("invalid selector format: %s, expected #(id=='value') format", selector)
		}
		id = matches[1]
		fieldPath = strings.TrimPrefix(matches[2], ".")
		if id == "" {
			return "", "", fmt.Errorf("ID cannot be empty in selector: %s", selector)
		}
		return id, fieldPath, nil
	}

	// Check for invalid # selectors
	if strings.HasPrefix(selector, "#") {
		return "", "", fmt.Errorf("invalid selector format: %s, expected #(id=='value') format", selector)
	}

	// Simple parsing for direct selectors - treat first dot as separator
	if strings.Contains(selector, ".") {
		parts := strings.SplitN(selector, ".", 2)
		return parts[0], parts[1], nil
	}

	return selector, "", nil
}

// ApplyFieldPath applies a GJSON field path to a configuration object
func ApplyFieldPath(config ref.Node, fieldPath string) (ref.Node, error) {
	// Marshal config to JSON for GJSON processing
	jsonBytes, err := marshalToJSON(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config for field access: %w", err)
	}

	// Use GJSON to extract the field
	result := gjson.GetBytes(jsonBytes, fieldPath)
	if !result.Exists() {
		return nil, core.NewError(nil, "FIELD_NOT_FOUND", map[string]any{
			"field_path": fieldPath,
		})
	}

	// Parse the result back to a Node
	return parseJSONValue(&result)
}

// marshalToJSON converts a Node to JSON bytes
func marshalToJSON(node ref.Node) ([]byte, error) {
	// Handle different node types for efficient marshaling
	switch v := node.(type) {
	case map[string]any, []any, string, int, int64, float64, bool, nil:
		// These types can be marshaled directly
		return json.Marshal(v)
	default:
		// For other types, try direct marshaling
		return json.Marshal(node)
	}
}

// parseJSONValue converts a gjson.Result to a ref.Node
func parseJSONValue(result *gjson.Result) (ref.Node, error) {
	switch result.Type {
	case gjson.String:
		return result.String(), nil
	case gjson.Number:
		// Try int first, then float
		if result.Int() == int64(result.Float()) {
			return result.Int(), nil
		}
		return result.Float(), nil
	case gjson.True:
		return true, nil
	case gjson.False:
		return false, nil
	case gjson.Null:
		return nil, nil
	case gjson.JSON:
		// Parse complex JSON structures
		var node any
		if err := json.Unmarshal([]byte(result.Raw), &node); err != nil {
			return nil, fmt.Errorf("failed to parse JSON value: %w", err)
		}
		return node, nil
	default:
		return nil, fmt.Errorf("unsupported gjson result type: %v", result.Type)
	}
}
