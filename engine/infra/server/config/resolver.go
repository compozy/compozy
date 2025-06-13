package config

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/pkg/ref"
)

// autoloadResourceResolver implements ref.ResourceResolver for autoloaded configurations
type autoloadResourceResolver struct {
	registry *autoload.ConfigRegistry
}

// ResolveResource resolves resource references using the autoload registry
func (r *autoloadResourceResolver) ResolveResource(resourceType, selector string) (ref.Node, error) {
	// Extract ID from selector like #(id=='task1').outputs
	id, drillDownPath, err := parseResourceSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resource selector: %w", err)
	}

	// Get configuration from registry
	config, err := r.registry.Get(resourceType, id)
	if err != nil {
		return nil, fmt.Errorf("resource not found: %w", err)
	}

	// Apply drill-down path if present (simple field access)
	if drillDownPath != "" {
		result, err := r.navigatePath(config, drillDownPath)
		if err != nil {
			return nil, fmt.Errorf("failed to navigate path %q: %w", drillDownPath, err)
		}
		return result, nil
	}

	// Return config directly as Node (Node is just 'any')
	return config, nil
}

// navigatePath performs simple path navigation for fields like ".outputs"
func (r *autoloadResourceResolver) navigatePath(data any, path string) (any, error) {
	if !strings.HasPrefix(path, ".") {
		return nil, fmt.Errorf("path must start with dot: %s", path)
	}

	fieldName := path[1:] // Remove leading dot

	// Convert to map if possible
	if m, ok := data.(map[string]any); ok {
		if value, exists := m[fieldName]; exists {
			return value, nil
		}
		return nil, fmt.Errorf("field %q not found", fieldName)
	}

	return nil, fmt.Errorf("cannot navigate path on non-map data")
}

// parseResourceSelector parses selector like #(id=='task1').outputs
func parseResourceSelector(selector string) (id string, drillDownPath string, err error) {
	// Find the ID within the selector
	if strings.HasPrefix(selector, "#(id==") {
		endIdx := strings.Index(selector, ")")
		if endIdx == -1 {
			return "", "", fmt.Errorf("invalid selector format: %s", selector)
		}

		// Extract ID value, removing quotes
		idPart := selector[6:endIdx] // Skip "#(id=="
		id = strings.Trim(idPart, "'\"")

		// Extract drill-down path if present
		if endIdx+1 < len(selector) {
			drillDownPath = selector[endIdx+1:]
		}

		return id, drillDownPath, nil
	}

	return "", "", fmt.Errorf("unsupported selector format: %s", selector)
}
