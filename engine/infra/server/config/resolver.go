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
	// Extract ID from selector using canonical parser
	id, drillDownPath, err := autoload.ParseResourceSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resource selector: %w", err)
	}

	// Get configuration from registry
	config, err := r.registry.Get(resourceType, id)
	if err != nil {
		return nil, err
	}

	// Apply drill-down path if present using robust field path resolution
	if drillDownPath != "" {
		// Remove leading dot for GJSON compatibility (it expects "field.subfield" not ".field.subfield")
		fieldPath := strings.TrimPrefix(drillDownPath, ".")
		result, err := autoload.ApplyFieldPath(config, fieldPath)
		if err != nil {
			return nil, fmt.Errorf("failed to navigate path %q: %w", drillDownPath, err)
		}
		return result, nil
	}

	// Return config directly as Node (Node is just 'any')
	return config, nil
}
