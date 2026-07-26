package extensionpkg

import "github.com/compozy/agh/internal/resources"

func resourceKindStrings(values []resources.ResourceKind) []string {
	converted := make([]string, 0, len(values))
	for _, value := range values {
		converted = append(converted, string(value))
	}
	return converted
}

func resourceScopeStrings(values []resources.ResourceScopeKind) []string {
	converted := make([]string, 0, len(values))
	for _, value := range values {
		converted = append(converted, string(value))
	}
	return converted
}
